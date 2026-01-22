package version

import "fmt"

// Cmd shows version information
type Cmd struct {
}

// Execute prints the version information
func Execute(c *Cmd, version string) error {
	fmt.Printf("dbmate-deployer version %s\n", version)
	return nil
}
