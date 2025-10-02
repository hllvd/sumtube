package sshpusher

import (
	"fmt"
	"os"
	"os/exec"
)

const remotePath = "/home/sumtube/sumtube/BE/renderer/static"

func PushSitemap(lang string, content string) error {
    host := os.Getenv("SSH_HOST")
    user := os.Getenv("SSH_USER")
    keyPath := os.Getenv("SSH_KEY_PATH")
	destinationPath := os.Getenv("DESTINATION_PATH")
    
    if host == "" || user == "" || keyPath == "" {
        return fmt.Errorf("missing required SSH configuration in environment")
    }
    
    // Create temporary file
    tmpFile, err := os.CreateTemp("", fmt.Sprintf("sitemap-%s-*.xml", lang))
    if err != nil {
        return fmt.Errorf("failed to create temp file: %v", err)
    }
    defer os.Remove(tmpFile.Name())
    
    // Write content to temp file
    if _, err := tmpFile.WriteString(content); err != nil {
        return fmt.Errorf("failed to write to temp file: %v", err)
    }
    
    // Close the file before copying
    if err := tmpFile.Close(); err != nil {
        return fmt.Errorf("failed to close temp file: %v", err)
    }
    
    // Construct scp command
    destPath := fmt.Sprintf("%s@%s:%s/sitemap-%s.xml", user, host,destinationPath, lang)
    cmd := exec.Command("scp", tmpFile.Name(), destPath)
    
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("scp failed: %v, output: %s", err, string(output))
    }
    
    return nil
}

