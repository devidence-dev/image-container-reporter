package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/user/docker-image-reporter/internal/registry"
	"github.com/user/docker-image-reporter/pkg/types"
	"github.com/user/docker-image-reporter/pkg/utils"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("usage: check_tags <repository> <current-tag>")
		os.Exit(2)
	}

	repo := os.Args[1]
	current := os.Args[2]

	client := registry.NewDockerHubClient(30 * time.Second)
	ctx := context.Background()

	image := types.DockerImage{Registry: "docker.io", Repository: repo, Tag: current}
	tags, err := client.GetLatestTags(ctx, image)
	if err != nil {
		log.Fatalf("GetLatestTags error: %v", err)
	}

	fmt.Printf("Retrieved %d tags for %s\n", len(tags), repo)
	// Print some alpine tags
	for _, t := range tags {
		if strings.Contains(strings.ToLower(t), "alpine") {
			fmt.Printf("alpine tag: %s  pre-release=%v\n", t, utils.IsPreRelease(t))
		}
	}

	// Also show what FilterTagsBySuffix would return when applied to stable tags
	stable := utils.FilterPreReleases(tags)
	if len(stable) == 0 {
		fmt.Println("No stable tags after FilterPreReleases()")
	} else {
		fmt.Printf("Stable tags count: %d\n", len(stable))
	}

	suffixFiltered := utils.FilterTagsBySuffix(stable, current)
	fmt.Printf("Suffix filtered count: %d\n", len(suffixFiltered))
	if len(suffixFiltered) > 0 {
		for _, s := range suffixFiltered {
			fmt.Printf(" suffix-> %s\n", s)
		}
	}

	best := utils.FindBestUpdateTag(current, tags)
	fmt.Printf("Best tag for %s (%s): %s\n", repo, current, best)
}
