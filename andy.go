package main

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"

	_ "github.com/denisenkom/go-mssqldb"
)

const fileExt = ".sql"

var foldersOrdered = []string{"tables", "procs", "funcs", "views", "triggers"}

func main() {
	checkFlag := flag.Bool("check", false, "Check for unexpected folder names")
	dbString := flag.String("dbstring", "", "MSSQL connection string in the format: sqlserver://user:password@dbname:1433?database=master")
	readStdin := flag.Bool("i", false, "Read file list from stdin")
	dryRunFlag := flag.Bool("dry-run", false, "Show files that will be processed without actual processing")
	flag.Parse()

	var sqlFiles []string
	if *readStdin {
		scanner := bufio.NewScanner(os.Stdin)

		// Read lines from stdin
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasSuffix(line, fileExt) {
				sqlFiles = append(sqlFiles, line)
			}
		}

		if err := scanner.Err(); err != nil {
			fmt.Println("Error reading input:", err)
			os.Exit(1)
		}
	}
	if len(sqlFiles) == 0 {
		fmt.Println("Error: No files provided. Exit")
		os.Exit(1)
	}

	fmt.Println("Files to process:")
	for _, folder := range foldersOrdered {
		fmt.Printf("%s:\n", folder)
		filesInFolder := filterFilesByFolder(sqlFiles, folder)
		sort.Strings(filesInFolder)
		for _, file := range filesInFolder {
			fmt.Printf("- %s\n", file)
		}
	}
	fmt.Println()

	var unexpectedFolders []string
	for _, file := range sqlFiles {
		parts := strings.Split(file, "/")
		if len(parts) >= 2 {
			folderName := parts[0]
			if !contains(foldersOrdered, folderName) {
				unexpectedFolders = appendIfMissing(unexpectedFolders, folderName)
			}
		}
	}

	// Print unexpected folder names and available options
	if len(unexpectedFolders) > 0 || *checkFlag {
		fmt.Println("ERROR: Unexpected folder names found!")
		for _, folder := range unexpectedFolders {
			fmt.Printf("- %s\n", folder)
		}
		fmt.Println("Allowed folder names:")
		for _, option := range foldersOrdered {
			fmt.Printf("- %s\n", option)
		}
		if *checkFlag {
			os.Exit(1)
		}
	}

	orderedFiles := reorderFiles(foldersOrdered, sqlFiles)
	executor := dryRunExecutor()
	if *dryRunFlag {
		fmt.Println("DRY-RUN MODE. Show files that will be processed without actual processing")
	} else {
		fmt.Println("REAL MODE. Run scripts with a specified DB connection")
		if *dbString == "" {
			fmt.Println("Error: --dbstring param not specified")
			os.Exit(1)
		}

		// Parse dbString as URL
		u, err := url.Parse(*dbString)
		if err != nil {
			fmt.Println("Error parsing dbstring:", err)
			os.Exit(1)
		}
		// Extract connection parameters from the URL
		server := u.Hostname()
		port := u.Port()
		user := u.User.Username()
		password, _ := u.User.Password()
		database := u.Query().Get("database")

		// Create the database connection
		connString := fmt.Sprintf("server=%s;port=%s;user id=%s;password=%s;database=%s", server, port, user, password, database)
		db, err := sql.Open("mssql", connString)
		if err != nil {
			fmt.Printf("Error: connection error: %s\n", err)
			os.Exit(1)
		}

		if err := db.Ping(); err != nil {
			fmt.Printf("Error: DB connect failed. %s\n", err)
			os.Exit(1)
		}

		defer db.Close()
		executor = sqlExecutor(db)
	}

	fmt.Println()
	fmt.Println("Run scripts...")
	runScripts(executor, orderedFiles)
	fmt.Println("Completed. OK.")
}

type scriptExecutorFunc func(script string) error

func sqlExecutor(db *sql.DB) scriptExecutorFunc {
	return func(script string) error {
		_, err := db.Exec(script)
		return err
	}
}

func dryRunExecutor() scriptExecutorFunc {
	return func(script string) error {
		return nil
	}
}

func runScripts(exec scriptExecutorFunc, sqlFiles []string) {
	for _, file := range sqlFiles {
		fmt.Printf("- Process file: %s\n", file)
		sqlBytes, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("Error reading SQL script file %s: %s\n", file, err)
			os.Exit(1)
		}

		err = exec(string(sqlBytes))
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			os.Exit(1)
		}
	}
}

func reorderFiles(foldersOrdered []string, sqlFiles []string) []string {
	var orderedFiles []string
	for _, folder := range foldersOrdered {
		filesInFolder := filterFilesByFolder(sqlFiles, folder)
		orderedFiles = append(orderedFiles, filesInFolder...)
	}
	return orderedFiles
}

func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

func appendIfMissing(slice []string, str string) []string {
	for _, s := range slice {
		if s == str {
			return slice
		}
	}
	return append(slice, str)
}

func filterFilesByFolder(files []string, folder string) []string {
	var filteredFiles []string
	for _, file := range files {
		parts := strings.Split(file, "/")
		if len(parts) >= 2 && parts[0] == folder {
			filteredFiles = append(filteredFiles, file)
		}
	}
	return filteredFiles
}
