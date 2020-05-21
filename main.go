package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"gopkg.in/yaml.v2"
)

type imageset struct {
	Name     string `yaml:"name"`
	Registry struct {
		Source      string `yaml:"source"`
		Destination string `yaml:"destination"`
	} `yaml:"registry"`
	Namespace struct {
		Source      string `yaml:"source"`
		Destination string `yaml:"destination"`
	} `yaml:"namespace"`
	Images []string `yaml:"images"`
}

var (
	targets map[string]imageset
	registrySource string
	registryDestination string

	configFile     string
	dry            bool
	imageSet       string
	tagSource      string
	tagDestiantion string
)

// retag will apply a new tag to an existing image reference.
func retag(origin, result string) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	cli.NegotiateAPIVersion(ctx)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("docker tag", origin, result)
	if !dry {
		if e := cli.ImageTag(ctx, origin, result); e != nil {
			fmt.Println(e)
			os.Exit(1)
		}
	}
}

// push will push an image to the destination location.
func push(result string) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	cli.NegotiateAPIVersion(ctx)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("docker push", result)
	if !dry {
		if _, e := cli.ImagePush(ctx, result, types.ImagePushOptions{}); e != nil {
			fmt.Println(e)
			os.Exit(1)
		}
	}
}

// pull will pull a given image into the local registry.
func pull(image, tag string) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	cli.NegotiateAPIVersion(ctx)
	if err != nil {
		fmt.Println(err)
	}

	images, e := cli.ImageList(ctx, types.ImageListOptions{})
	if e != nil {
		fmt.Println(e)
		os.Exit(1)
	}
	for n := range images {
		for t := range images[n].RepoTags {
			if images[n].RepoTags[t] == image+":"+tag {
				fmt.Printf("# docker pull %v:%v\n", image, tag)
				return
			}
		}
	}

	ref := fmt.Sprintf("%v:%v", image, tag)
	fmt.Println("docker pull", ref)
	if !dry {
		if _, e := cli.ImagePull(ctx, ref, types.ImagePullOptions{}); e != nil {
			fmt.Println(e)
			os.Exit(1)
		} else {
			fmt.Println(e)
			os.Exit(1)
		}
	}
}

// diff will handle container diffs
// TODO container diffs
func diff(origin, result string) {}

// config will handle the marshalling the config file
func config() {
	b, err := ioutil.ReadFile(configFile)
	if err != nil {
		fmt.Print(err)
	}

	e := yaml.Unmarshal(b, &targets)
	if e != nil {
		fmt.Println(e)
	}

	for target := range targets {
		if targets[target].Registry.Source == "" {
			registrySource = "docker.io"
		} else {
			registrySource = targets[target].Registry.Source
		}
		if targets[target].Registry.Destination == "" {
			registryDestination = registrySource
		} else {
			registryDestination = targets[target].Registry.Destination
		}
	}
}

// main is the entrypoint to this application.
func main() {

	flag.StringVar(&configFile, "config", "config.yml", "Path to configuration file")
	flag.BoolVar(&dry, "dry-run", false, "Do not perform any actions, just report the expected actions")
	flag.StringVar(&imageSet, "set", "", "Run the workload against the specified image-set")
	flag.StringVar(&tagSource, "source", "", "Source tag to identify or pull before processing")
	flag.StringVar(&tagDestiantion, "destination", "", "Destination tag to push to")

	flag.Parse()

	// Unmarshal yaml config.
	config()

	if imageSet == "" || targets[imageSet].Name == "" {
		fmt.Println("missing flag 'set' for configuration item to choose")
	}

	if tagSource == "" {
		fmt.Println("missing flag 'source' for input tag reference")
	}

	if tagDestiantion == "" {
		fmt.Println("missing flag 'destination' for input tag reference")
	}

	if (imageSet == "" || targets[imageSet].Name == "") || tagSource == "" || tagDestiantion == "" {
		os.Exit(1)
	}

	// Ensure images exist:
	Item := targets[imageSet]
	for _, image := range Item.Images {
		pull(registrySource + "/" + Item.Namespace.Source+"/"+image, tagSource)
	}

	// Retag images:
	for _, image := range Item.Images {
		retag(registrySource + "/" + Item.Namespace.Source+"/"+image+":"+tagSource, registryDestination + "/" + Item.Namespace.Destination+"/"+image+":"+tagDestiantion)
	}

	// Push images:
	for _, image := range Item.Images {
		push(registryDestination + "/" + Item.Namespace.Destination + "/" + image + ":" + tagDestiantion)
	}
}
