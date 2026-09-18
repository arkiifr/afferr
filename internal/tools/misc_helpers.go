package tools

import "os"

func writeFile0600(path string, data []byte) error { return os.WriteFile(path, data, 0o600) }
func remove(path string) error { return os.Remove(path) }
