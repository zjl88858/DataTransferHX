package core

import (
	"fmt"
	"io"
	"log"
	"path"
	"regexp"
	"time"

	"filetransferhx/config"
	"filetransferhx/protocols"
)

type TransferManager struct {
	HistoryManager *HistoryManager
}

func NewTransferManager(hm *HistoryManager) *TransferManager {
	return &TransferManager{
		HistoryManager: hm,
	}
}

func (tm *TransferManager) RunTask(task config.Task) error {
	log.Printf("Starting task: %s", task.Name)

	// 1. Init FileSystems
	srcFS, err := tm.createFileSystem(task.SourceType, task.SourcePath, task.SourceAuth)
	if err != nil {
		return fmt.Errorf("failed to init source fs: %v", err)
	}
	defer srcFS.Close()

	dstFS, err := tm.createFileSystem(task.TargetType, task.TargetPath, task.TargetAuth)
	if err != nil {
		return fmt.Errorf("failed to init target fs: %v", err)
	}
	defer dstFS.Close()

	// 2. Load History
	history := tm.HistoryManager.GetTaskHistory(task.Name)

	// 3. Traverse and Transfer
	err = tm.processDirectory(srcFS, dstFS, "", task, history)
	if err != nil {
		log.Printf("Error processing directory for task %s: %v", task.Name, err)
		// Continue to cleanup even if transfer failed partially
	}

	// 4. Cleanup
	if task.RetentionDays > 0 {
		tm.cleanup(dstFS, task, history)
	}

	// 5. Save History
	tm.HistoryManager.Save()
	log.Printf("Finished task: %s", task.Name)
	return nil
}

func (tm *TransferManager) createFileSystem(fsType, rootPath string, auth *config.Auth) (protocols.FileSystem, error) {
	switch fsType {
	case "local":
		fs := &protocols.LocalFileSystem{RootPath: rootPath}
		return fs, fs.Init()
	case "sftp":
		if auth == nil {
			return nil, fmt.Errorf("auth required for sftp")
		}
		fs := &protocols.SFTPFileSystem{
			Host:     auth.Host,
			Port:     auth.Port,
			User:     auth.User,
			Password: auth.Password,
			RootPath: rootPath,
		}
		return fs, fs.Init()
	case "ftp":
		if auth == nil {
			return nil, fmt.Errorf("auth required for ftp")
		}
		fs := &protocols.FTPFileSystem{
			Host:     auth.Host,
			Port:     auth.Port,
			User:     auth.User,
			Password: auth.Password,
			RootPath: rootPath,
		}
		return fs, fs.Init()
	default:
		return nil, fmt.Errorf("unknown fs type: %s", fsType)
	}
}

func (tm *TransferManager) processDirectory(srcFS, dstFS protocols.FileSystem, relPath string, task config.Task, history *TaskHistory) error {
	entries, err := srcFS.List(relPath)
	if err != nil {
		return err
	}

	regex, err := regexp.Compile(task.SourceRegex)
	if err != nil {
		return fmt.Errorf("invalid regex: %v", err)
	}

	for _, entry := range entries {
		entryRelPath := path.Join(relPath, entry.Name)

		if entry.IsDir {
			// Recursion
			err := tm.processDirectory(srcFS, dstFS, entryRelPath, task, history)
			if err != nil {
				log.Printf("Error processing subdir %s: %v", entryRelPath, err)
			}
			continue
		}

		// Filter
		if !regex.MatchString(entry.Name) {
			continue
		}

		// Filter by ModTime
		if task.SourceNewerDays > 0 {
			cutoff := time.Now().AddDate(0, 0, -task.SourceNewerDays)
			if entry.ModTime.Before(cutoff) {
				continue
			}
		}

		// Check History
		if history.Has(entryRelPath) {
			// Already transferred
			// Optional: Check ModTime or Size to see if it changed?
			// For now, strict "avoid duplicate" based on history record.
			continue
		}

		// Transfer
		err := tm.transferFile(srcFS, dstFS, entryRelPath)
		if err != nil {
			log.Printf("Failed to transfer %s: %v", entryRelPath, err)
			continue
		}
		log.Printf("Transferred file: %s (size: %d)", entryRelPath, entry.Size)

		// Update History
		history.Add(entryRelPath)
	}
	return nil
}

func (tm *TransferManager) transferFile(srcFS, dstFS protocols.FileSystem, relPath string) error {
	// Ensure parent dir exists in target
	parentDir := path.Dir(relPath)
	if parentDir != "." && parentDir != "/" {
		err := dstFS.MkdirAll(parentDir)
		if err != nil {
			return fmt.Errorf("failed to mkdir %s: %v", parentDir, err)
		}
	}

	// Open Source
	srcFile, err := srcFS.Open(relPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Create Target
	dstFile, err := dstFS.Create(relPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// Copy
	_, err = io.Copy(dstFile, srcFile)
	return err
}

func (tm *TransferManager) cleanup(dstFS protocols.FileSystem, task config.Task, history *TaskHistory) {
	cutoff := time.Now().AddDate(0, 0, -task.RetentionDays)

	history.mu.RLock()
	records := make(map[string]time.Time, len(history.Records))
	for k, v := range history.Records {
		records[k] = v
	}
	history.mu.RUnlock()

	for relPath, transferTime := range records {
		if transferTime.Before(cutoff) {
			// Check if file exists before trying to delete
			_, err := dstFS.Stat(relPath)
			if err != nil {
				// File likely doesn't exist, skip
				continue
			}

			log.Printf("Cleaning up old file: %s (transferred at %v)", relPath, transferTime)
			err = dstFS.Remove(relPath)
			if err != nil {
				log.Printf("Failed to remove %s: %v", relPath, err)
			}
		}
	}
}
