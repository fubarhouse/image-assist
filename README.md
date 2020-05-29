# Image Assist

## Background
This little tool is an open sourced conversion of a bash script I've been maintaining for local release management for Docker images.

The script will retag and push to the destiantion tag location, but this version of the script has a few nicer features which make it a bit safer.

Even though that script will not be public, the differences include:
* Everything is declared in a config file.
* Images will now be checked if they're in the local registry and will be pulled in if they're absent.
* Image tags can be pushed to different namespaces on the same registry.
* There is an optional `dry-run` flag which will help prevent any unwanted action from occuring.

## Usage
```shell script
$ go run main.go -h                                                     
  -config string
        Path to configuration file (default "config.yml")
  -destination string
        Destination tag to push to
  -diff
        In the cases where dry-run is enabled, also run the diff action
  -dry-run
        Do not perform any actions, just report the expected actions
  -exit-on-fail
        Exit on failure of any Docker API call.
  -pull
        In the cases where dry-run is enabled, also run the pull action
  -push
        In the cases where dry-run is enabled, also run the push action
  -retag
        In the cases where dry-run is enabled, also run the retag action
  -set string
        Run the workload against the specified image-set
  -source string
        Source tag to identify or pull before processing
exit status 2 
```

### Flag explanation
* `-config` will allow you to change the path to the configuration file, and an example has been provided to show the structure.
* `-destination` is the destination tag onto the new/existing namespace as configured.
* `-dry-run` will prevent any Docker images from being pulled, retagged or pushed.
* `-set` Is the map label to use as identified from the configuration file.
* `-source` is the source tag on the Docker image to identify or pull from. 

## License

This project has an MIT license, please use this at your own risk.