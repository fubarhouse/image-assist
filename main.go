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
	Name       string              `yaml:"name"`
	Read       string              `yaml:"read"`
	Write      string              `yaml:"write"`
	Registries map[string]Registry `yaml:"registries"`
	Images     []string            `yaml:"images"`
}

type Registry struct {
	URL       string `yaml:"url"`
	Auth      string `yaml:"auth"`
	Namespace string `yaml:"namespace"`
}

var (
	targets map[string]imageset

	configFile     string
	dry            bool
	imageSet       string
	tagSource      string
	tagDestiantion string

	exitOnFail bool

	diffAction  bool
	pullAction  bool
	pushAction  bool
	retagAction bool
)

// retag will apply a new tag to an existing image reference.
//func retag(origin, result string) {
func retag(registryOne, registryTwo string, TagOne, TagTwo, Image string) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	cli.NegotiateAPIVersion(ctx)
	if err != nil {
		fmt.Println(err)
	}

	origin := fmt.Sprintf("%v/%v:%v", targets[imageSet].Registries[targets[imageSet].Read].Namespace, Image, TagOne)
	result := fmt.Sprintf("%v/%v/%v:%v", targets[imageSet].Registries[targets[imageSet].Write].URL, targets[imageSet].Registries[targets[imageSet].Write].Namespace, Image, TagTwo)

	fmt.Println("docker tag", origin, result)
	if dry && !retagAction {
		return
	}

	if e := cli.ImageTag(ctx, origin, result); e != nil {
		fmt.Println("", e)
		if exitOnFail {
			os.Exit(1)
		}
	}
}

// push will push an image to the destination location.
func push(Registry Registry, Image, Tag string) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	cli.NegotiateAPIVersion(ctx)
	if err != nil {
		fmt.Println(err)
	}

	ref := fmt.Sprintf("%v/%v/%v:%v", Registry.URL, Registry.Namespace, Image, Tag)

	fmt.Println("docker push", ref)
	if dry && !pushAction {
		return
	}

	if _, e := cli.ImagePush(ctx, ref, types.ImagePushOptions{
		RegistryAuth: Registry.Auth,
	}); e != nil {
		fmt.Println("", e)
		if exitOnFail {
			os.Exit(1)
		}
	}
}

// pull will pull a given image into the local registry.
func pull(r Registry, image, tag string) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	cli.NegotiateAPIVersion(ctx)
	if err != nil && exitOnFail {
		fmt.Println(err)
	}

	images, e := cli.ImageList(ctx, types.ImageListOptions{})
	if e != nil {
		fmt.Println(e)
		if exitOnFail {
			os.Exit(1)
		}
	}

	ref := fmt.Sprintf("%v/%v/%v:%v", r.URL, r.Namespace, image, tag)

	for n := range images {
		for t := range images[n].RepoTags {
			if strings.Contains(image+":"+tag, images[n].RepoTags[t]) {
				fmt.Printf("# docker pull %v\n", ref)
				return
			}
		}
	}

	fmt.Printf("docker pull %v\n", ref)

	if dry && !pullAction {
		return
	}

	if _, e := cli.ImagePull(ctx, ref, types.ImagePullOptions{
		RegistryAuth: r.Auth,
	}); e != nil {
		fmt.Println("", e)
		if exitOnFail {
			os.Exit(1)
		}
	}
}

// diff will handle container diffs
// Here, we're using container-diff:
// https://github.com/GoogleContainerTools/container-diff
func diff(registryOne, registryTwo string, TagOne, TagTwo, Image string) {
	origin := fmt.Sprintf("remote://%v/%v/%v:%v", targets[imageSet].Registries[targets[imageSet].Read].URL, targets[imageSet].Registries[targets[imageSet].Read].Namespace, Image, TagOne)
	result := fmt.Sprintf("daemon://%v/%v/%v:%v", targets[imageSet].Registries[targets[imageSet].Write].URL, targets[imageSet].Registries[targets[imageSet].Write].Namespace, Image, TagTwo)
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
	output, commandErr := exec.Command(binPath, "diff", origin, result, "--type=file").Output()

	if commandErr != nil {
		fmt.Printf(" %v\n", commandErr.Error())
		return
	}

	fmt.Printf(" ... done.\n")

	os.Remove(fileName)
	f, err := os.Create(fileName)
	defer f.Close()
	if err != nil {
		fmt.Println(err)
		if exitOnFail {
			os.Exit(1)
		}
	}
	_, err = f.WriteString(string(output))
	if err != nil {
		fmt.Println(err)
		if exitOnFail {
			os.Exit(1)
		}
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
}

// main is the entrypoint to this application.
func main() {

	flag.StringVar(&configFile, "config", "config.yml", "Path to configuration file")
	flag.BoolVar(&dry, "dry-run", false, "Do not perform any actions, just report the expected actions")
	flag.StringVar(&imageSet, "set", "", "Run the workload against the specified image-set")
	flag.StringVar(&tagSource, "source", "", "Source tag to identify or pull before processing")
	flag.StringVar(&tagDestiantion, "destination", "", "Destination tag to push to")

	flag.BoolVar(&exitOnFail, "exit-on-fail", false, "Exit on failure of any Docker API call.")

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

	// Loop over all the targets:
	for n, r := range targets[imageSet].Registries {
		// Ensure images exist:
		if targets[imageSet].Read == n {
			for _, image := range targets[imageSet].Images {
				pull(r, image, tagSource)
			}
		}
	}

	// Retag images:
	for _, image := range targets[imageSet].Images {
		retag(targets[imageSet].Read, targets[imageSet].Write, tagSource, tagDestiantion, image)
		// registrySource+"/"+namespaceSource+"/"+image+":"+tagSource, registryDestination+"/"+namespaceDestination+"/"+image+":"+tagDestiantion
	}

	// Diff images:
	for _, image := range targets[imageSet].Images {
		diff(targets[imageSet].Read, targets[imageSet].Write, tagSource, tagDestiantion, image)
	}

	// Push images:
	for n, r := range targets[imageSet].Registries {
		if targets[imageSet].Read == n {
			for _, image := range targets[imageSet].Images {
				push(r, image, tagDestiantion)
			}
		}
	}
}
