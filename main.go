package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

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
	targets             map[string]imageset
	registrySource      string
	registryDestination string

	configFile           string
	dry                  bool
	imageSet             string
	namespaceDestination string
	tagSource            string
	tagDestiantion       string

	diffAction  bool
	pullAction  bool
	pushAction  bool
	retagAction bool
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
	if dry && !retagAction {
		return
	}

	if e := cli.ImageTag(ctx, origin, result); e != nil {
		fmt.Println(e)
		os.Exit(1)
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
	if dry && !pushAction {
		return
	}

	if _, e := cli.ImagePush(ctx, result, types.ImagePushOptions{}); e != nil {
		fmt.Println(e)
		os.Exit(1)
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
			if strings.Contains(image+":"+tag, images[n].RepoTags[t]) {
				fmt.Printf("# docker pull %v/%v:%v\n", registrySource, image, tag)
				return
			}
		}
	}

	ref := fmt.Sprintf("%v/%v:%v", registrySource, image, tag)
	fmt.Println("docker pull", ref)

	if dry && !pullAction {
		return
	}

	if _, e := cli.ImagePull(ctx, ref, types.ImagePullOptions{}); e != nil {
		fmt.Println(e)
		os.Exit(1)
	}
}

// diff will handle container diffs
// Here, we're using container-diff:
// https://github.com/GoogleContainerTools/container-diff
func diff(origin, result string) {
	binPath, err := exec.LookPath("container-diff")
	if err != nil || binPath == "" {
		return
	}
	fmt.Printf("%v diff %v %v --type=file", binPath, origin, result)

	if dry && !diffAction {
		fmt.Printf(" ... not doing.\n")
		return
	}

	parts := strings.Split(result, "/")
	lastPart := parts[len(parts)-1]
	tagName := strings.Split(lastPart, ":")[0]
	imageName := strings.Split(lastPart, ":")[1]
	fileName := fmt.Sprintf("container-diff_%v_%v_%v.txt", imageSet, imageName, tagName)
	output, commandErr := exec.Command(binPath, "diff", result, origin, "--type=file").Output()

	if commandErr != nil {
		fmt.Printf("%v\n", commandErr.Error())
		return
	}

	fmt.Printf(" ... done.\n")

	os.Remove(fileName)
	f, err := os.Create(fileName)
	defer f.Close()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	_, err = f.WriteString(string(output))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

}

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
		if targets[target].Namespace.Destination == "" {
			namespaceDestination = targets[target].Namespace.Source
		} else {
			namespaceDestination = targets[target].Namespace.Destination
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

	flag.BoolVar(&diffAction, "diff", false, "In the cases where dry-run is enabled, also run the diff action")
	flag.BoolVar(&pullAction, "pull", false, "In the cases where dry-run is enabled, also run the pull action")
	flag.BoolVar(&pushAction, "push", false, "In the cases where dry-run is enabled, also run the push action")
	flag.BoolVar(&retagAction, "retag", false, "In the cases where dry-run is enabled, also run the retag action")

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
		pull(Item.Namespace.Source+"/"+image, tagSource)
	}

	// Retag images:
	for _, image := range Item.Images {
		retag(registrySource+"/"+Item.Namespace.Source+"/"+image+":"+tagSource, registryDestination+"/"+namespaceDestination+"/"+image+":"+tagDestiantion)
	}

	// Diff images:
	for _, image := range Item.Images {
		diff(registrySource+"/"+Item.Namespace.Source+"/"+image+":"+tagSource, registryDestination+"/"+namespaceDestination+"/"+image+":"+tagDestiantion)
	}

	// Push images:
	for _, image := range Item.Images {
		push(registryDestination + "/" + namespaceDestination + "/" + image + ":" + tagDestiantion)
	}
}
