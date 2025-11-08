package main

import (
	"fmt"
	"sort"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

// stageResult holds the result of running a test stage
type stageResult struct {
	name string
	err  error
}

// runTestAll executes the complete test-all workflow:
// 1. Builds all artifacts defined in forge.yaml
// 2. Runs all test stages sequentially in order
// 3. Auto-deletes test environments after each stage
// 4. Continues execution even if stages fail
// 5. Returns aggregate error if any stage failed
//
// Usage: forge test-all
func runTestAll(args []string) error {
	// Step 1: Build all artifacts
	fmt.Println("🔨 Building all artifacts...")
	if err := runBuild([]string{}); err != nil {
		fmt.Printf("❌ Build failed: %v\n", err)
		return fmt.Errorf("build failed: %w", err)
	}
	fmt.Println("✅ Build completed successfully")

	// Step 2: Load configuration and discover test stages
	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load forge.yaml: %w", err)
	}

	// Check if there are any test stages
	if len(config.Test) == 0 {
		fmt.Println("\n⚠️  No test stages defined in forge.yaml")
		return nil
	}

	// Print test stage summary
	fmt.Printf("\n🧪 Running %d test stage(s)...\n", len(config.Test))

	// Step 3: Execute test stages
	var results []stageResult

	for i := range config.Test {
		testSpec := &config.Test[i]
		fmt.Printf("\n--- Running test stage: %s ---\n", testSpec.Name)

		// Execute the test stage
		err := testRun(&config, testSpec, []string{})

		// Store the result
		results = append(results, stageResult{name: testSpec.Name, err: err})

		// Print stage result
		if err == nil {
			fmt.Printf("✅ Stage '%s' passed\n", testSpec.Name)
		} else {
			fmt.Printf("❌ Stage '%s' failed: %v\n", testSpec.Name, err)
		}

		// Auto-delete test environment if one was created
		if testSpec.Testenv != "" && testSpec.Testenv != "noop" {
			if cleanupErr := cleanupTestEnvironment(&config, testSpec); cleanupErr != nil {
				// Log but don't fail - cleanup is best effort
				fmt.Printf("⚠️  Warning: Failed to cleanup test environment for stage '%s': %v\n", testSpec.Name, cleanupErr)
			} else {
				fmt.Printf("🧹 Cleaned up test environment for stage '%s'\n", testSpec.Name)
			}
		}
	}

	// Step 4: Print aggregate summary
	fmt.Println("\n=== Test Summary ===")

	// Count results
	totalStages := len(results)
	passedStages := 0
	failedStages := 0
	var failedStageDetails []stageResult

	for _, result := range results {
		if result.err == nil {
			passedStages++
		} else {
			failedStages++
			failedStageDetails = append(failedStageDetails, result)
		}
	}

	// Print summary counts
	fmt.Printf("Total stages: %d\n", totalStages)
	fmt.Printf("Passed: %d\n", passedStages)
	fmt.Printf("Failed: %d\n", failedStages)

	// Print detailed failure list if any
	if failedStages > 0 {
		fmt.Println("\nFailed stages:")
		for _, failed := range failedStageDetails {
			fmt.Printf("  - %s: %v\n", failed.name, failed.err)
		}
	}

	// Determine exit status
	if failedStages == 0 {
		fmt.Println("\n✅ All test stages passed!")
		return nil
	}

	fmt.Printf("\n❌ %d test stage(s) failed\n", failedStages)
	return fmt.Errorf("%d test stage(s) failed", failedStages)
}

// cleanupTestEnvironment deletes the most recent test environment for a stage
func cleanupTestEnvironment(config *forge.Spec, testSpec *forge.TestSpec) error {
	// Read artifact store
	store, err := forge.ReadOrCreateArtifactStore(config.ArtifactStorePath)
	if err != nil {
		return fmt.Errorf("failed to read artifact store: %w", err)
	}

	// Get all test environments for this stage
	envs := forge.ListTestEnvironments(&store, testSpec.Name)
	if len(envs) == 0 {
		// No environment to clean up (testRun may have failed before creating one)
		return nil
	}

	// Sort by CreatedAt descending to get the most recent first
	sort.Slice(envs, func(i, j int) bool {
		return envs[i].CreatedAt.After(envs[j].CreatedAt)
	})

	// Get the most recent environment
	mostRecent := envs[0]

	// Delete it using testDelete
	if err := testDelete(testSpec, []string{mostRecent.ID}); err != nil {
		return fmt.Errorf("failed to delete environment %s: %w", mostRecent.ID, err)
	}

	return nil
}
