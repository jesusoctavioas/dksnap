package snapshot

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

// Mongo creates snapshots for Mongo containers. It dumps the database
// using `mongodump`.
type Mongo struct {
	client *client.Client
}

// NewMongo creates a new mongo snapshotter.
func NewMongo(c *client.Client) Snapshotter {
	return &Mongo{c}
}

// Create creates a new snapshot.
func (c *Mongo) Create(ctx context.Context, container types.ContainerJSON, title, imageName string) error {
	buildContext, err := ioutil.TempDir("", "dksnap-context")
	if err != nil {
		return fmt.Errorf("make build context dir: %w", err)
	}
	defer os.RemoveAll(buildContext)

	dump, err := exec(ctx, c.client, container.ID, []string{"mongodump", "--archive"})
	if err != nil {
		return fmt.Errorf("dump: %w", err)
	}

	if err := ioutil.WriteFile(filepath.Join(buildContext, "dump.archive"), dump, 0644); err != nil {
		return fmt.Errorf("write dump: %w", err)
	}

	loadScript := []byte("mongorestore --drop --archive=/dksnap/dump.archive")
	if err := ioutil.WriteFile(filepath.Join(buildContext, "load-dump.sh"), loadScript, 0755); err != nil {
		return fmt.Errorf("write load script: %w", err)
	}

	err = buildImage(ctx, c.client, buildOptions{
		baseImage: container.Image,
		context:   buildContext,
		bootCommands: []string{
			"rm -rf /data/db/*",
		},
		buildInstructions: []string{
			"COPY dump.archive /dksnap/dump.archive",
			"COPY load-dump.sh /docker-entrypoint-initdb.d/load-dump.sh",
		},
		title:      title,
		imageNames: []string{imageName},
		dumpPath:   "/dksnap/dump.archive",
	})
	if err != nil {
		return fmt.Errorf("build image: %w", err)
	}
	return nil
}
