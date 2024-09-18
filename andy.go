package main

import (
	"bufio"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
	"github.com/ilyakaznacheev/cleanenv"
	"golang.org/x/exp/slices"
)

const fileExt = ".sql"

// buildRelease строка-идентификатор билда релиза.
// Задается из gitlab CI посредством передачи CI_COMMIT_REF_NAME с помощью ldflags.
// Пример использования при локальной сборке: go build -ldflags "-X main.buildRelease=$BUILD_RELEASE".
// nolint
var buildRelease string

// vcsRevision строка-идентификатор первых 8-ми символов CI_COMMIT_SHA.
// Задается из gitlab CI посредством передачи CI_COMMIT_SHORT_SHA с помощью ldflags.
// Пример использования при локальной сборке: go build -ldflags "-X main.vcsRevision=$COMMIT_SHORT_SHA".
// nolint
var vcsRevision string

// nolint
var (
	vcsBranch string
	buildDate string
)

func main() {
	configFile := flag.String("config", "config.yml", "path to config file")
	changesFileName := flag.String("f", "", "Read changed files list from a file")
	readStdin := flag.Bool("i", false, "Read changed files list from stdin")
	dryRunFlag := flag.Bool("dry-run", false, "Show files that will be processed without actual processing")
	rootDir := flag.String("dir", "", "Path to directory with databeses sql files")
	versionFlag := flag.Bool("version", false, "Show application version info")
	if len(os.Args) < 2 {
		os.Args = append(os.Args, "--help")
	}
	flag.Parse()

	if *versionFlag {
		fmt.Printf("Version: %s, VCS Revision: %s, Branch: %s, BuildDate: %s\n", buildRelease, vcsRevision, vcsBranch, buildDate)
		os.Exit(0)
	}

	cfg, err := loadConfig(configFile)
	if err != nil {
		fmt.Println("ERROR: Failed to load config file. Err: ", err)
		os.Exit(1)
	}

	if *dryRunFlag {
		fmt.Println("DRY-RUN MODE. Show files that will be processed without actual processing")
	} else {
		fmt.Println("REAL MODE. Run DB scripts")
	}

	if *rootDir != "" {
		fmt.Println("Using folder:", *rootDir)
	}
	fmt.Printf("Groups exec order: %v\n", cfg.ChangesOrder)

	if (*changesFileName != "") && *readStdin {
		fmt.Println("ERROR: You should use -i or -f flag, not both")
		os.Exit(2)
	}

	if *changesFileName == "" && !*readStdin {
		fmt.Println("ERROR: You must specify changes list, use -i or -f flag")
		os.Exit(2)
	}

	var changedFiles []string
	if *readStdin {
		var err error
		if changedFiles, err = readChangedFilesList(os.Stdin); err != nil {
			fmt.Println("Read changes error: ", err)
			os.Exit(1)
		}
	}
	if *changesFileName != "" {
		f, err := os.Open(*changesFileName)
		if err != nil {
			fmt.Println("Open changes file error: ", err)
			os.Exit(1)
		}
		defer f.Close()

		reader := bufio.NewReader(f)
		if changedFiles, err = readChangedFilesList(reader); err != nil {
			fmt.Println("Read changes error: ", err)
			os.Exit(1)
		}
	}

	if len(changedFiles) == 0 {
		fmt.Println("No changes found. Nothing to execute")
		os.Exit(0)
	}

	changeset, validationErrors, err := prepareChangeset(*rootDir, changedFiles, cfg.ChangesOrder)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	if len(validationErrors) > 0 {
		fmt.Printf("\n! Ignoring files with path validation errors:\n%s\n", validationErrorsAsString(validationErrors))
	}

	fmt.Println()

	if len(changeset) == 0 {
		fmt.Println("No valid changes to execute. Exiting")
		os.Exit(0)
	}

	fmt.Println("Run sql scripts...")

	if vr := checkDBConnectionSettings(changeset, *cfg); len(vr) != 0 {
		fmt.Printf("ERROR! Some DB connection settings missing:\n%s\n", validationErrorsAsString(vr))
		fmt.Println("Check the config file contents at ", *configFile)
		os.Exit(2)
	}

	if err := applyDatabaseChanges(context.Background(), cfg, changeset, *dryRunFlag); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	fmt.Println("Completed. OK.")
}

type (
	DBName     string
	GroupName  string
	SchemaName string
)

type DatabaseChangeset struct {
	GroupChanges map[GroupName]GroupChangeset
}

type GroupChangeset struct {
	SchemaChanges map[SchemaName]SchemaChangeset
}

type ObjectInfo struct {
	FileName     string
	RelativePath string
	FullPath     string
}
type SchemaChangeset []ObjectInfo // список путей файлов

type ValidationError struct {
	Object  string
	Message string
}

func (vr ValidationError) String() string {
	return vr.Object + ": " + vr.Message
}

func validationErrorsAsString(vr []ValidationError) string {
	var lines []string
	for _, ve := range vr {
		lines = append(lines, "- "+ve.String())
	}
	return strings.Join(lines, "\n")
}

func prepareChangeset(dir string, changedFiles []string, allowedGroupNames []string) (map[DBName]DatabaseChangeset, []ValidationError, error) {
	fail := func(err error) error {
		return fmt.Errorf("prepareChangeset: %w", err)
	}

	changeset := make(map[DBName]DatabaseChangeset)
	var ignoredFiles []ValidationError

	for _, file := range changedFiles {
		si, err := getScriptInfo(file)
		if err != nil {
			return nil, nil, fail(fmt.Errorf("Invalid path format: %w", err))
		}

		dbName := DBName(si.DatabaseName)
		groupName := GroupName(si.GroupType)
		schemaName := SchemaName(si.SchemaName)
		objFile := si.FileName

		if !slices.Contains(allowedGroupNames, string(groupName)) {
			ignoredFiles = append(ignoredFiles, ValidationError{
				Object:  file,
				Message: fmt.Sprintf("Group name '%s' is not valid! Valid names: %+v", groupName, allowedGroupNames),
			})
			continue
		}

		if _, found := changeset[dbName]; !found {
			changeset[dbName] = DatabaseChangeset{
				GroupChanges: map[GroupName]GroupChangeset{},
			}
		}

		dbChanges := changeset[dbName]
		if _, found := dbChanges.GroupChanges[groupName]; !found {
			dbChanges.GroupChanges[groupName] = GroupChangeset{
				SchemaChanges: map[SchemaName]SchemaChangeset{},
			}
		}

		groupChanges := dbChanges.GroupChanges[groupName]
		groupChanges.SchemaChanges[schemaName] = append(groupChanges.SchemaChanges[schemaName], ObjectInfo{FileName: objFile, RelativePath: file, FullPath: path.Join(dir, file)})
		dbChanges.GroupChanges[groupName] = groupChanges
		changeset[dbName] = dbChanges
	}

	return changeset, ignoredFiles, nil
}

func applyDatabaseChanges(ctx context.Context, cfg *AndyConfig, changeset map[DBName]DatabaseChangeset, dryRun bool) error {
	fail := func(err error) error {
		return err // fmt.Errorf("applyDatabaseChanges: %w", err)
	}

	for dbName, dbChanges := range changeset {
		fmt.Printf("= Database: %s =\n", dbName)

		dbConf, found := cfg.FindDatabse(dbName)
		if !found {
			return fail(fmt.Errorf("DB connection settings not found"))
		}

		if err := applyDBChanges(ctx, dbConf, time.Duration(dbConf.QueryExecTimeoutSec)*time.Second, dbChanges, cfg.ChangesOrder, dryRun); err != nil {
			return fail(err)
		}
		fmt.Println()
	}

	return nil
}

func applyDBChanges(ctx context.Context, dbConf DatabaseConfig, execTimeout time.Duration, dbChanges DatabaseChangeset, groupsChangeOrder []string, dryRun bool) error {
	fail := func(err error) error {
		return err // fmt.Errorf("applyDBChanges: %w", err)
	}

	var db *sql.DB
	if !dryRun {
		var err error
		db, err = getDBConnection(ctx, dbConf)
		if err != nil {
			return fail(fmt.Errorf("Get DB connection error: %w", err))
		}
	}

	for _, groupName := range groupsChangeOrder {
		fmt.Printf("  - groups: %s\n", groupName)
		dbGroupChanges := dbChanges.GroupChanges[GroupName(groupName)]
		if err := applyDBGroupChanges(ctx, db, execTimeout, dbGroupChanges, dryRun); err != nil {
			return fail(err)
		}
	}

	return nil
}

func applyDBGroupChanges(ctx context.Context, db *sql.DB, execTimeout time.Duration, dbGroupChanges GroupChangeset, dryRun bool) error {
	fail := func(err error) error {
		return err // fmt.Errorf("applyDBGroupChanges: %w", err)
	}

	for schema, schemaChanges := range dbGroupChanges.SchemaChanges {
		fmt.Printf("    - schema: %s\n", schema)

		orderedObjects := schemaChanges
		// Сортировка по алфавиту
		sort.Slice(orderedObjects, func(i, j int) bool {
			return orderedObjects[i].FileName < orderedObjects[j].FileName
		})

		for _, object := range orderedObjects {
			fmt.Printf("      - %s", object.RelativePath)

			if dryRun {
				fmt.Printf(" >> Exec skipped. DRY-RUN mode\n")
				continue
			}

			sqlText, err := getSQLText(object.FullPath)
			if err != nil {
				return fail(fmt.Errorf("File read error: %w", err))
			}
			if err := realScriptExec(ctx, db, execTimeout, sqlText); err != nil {
				return fail(fmt.Errorf("Exec script error: %w", err))
			}
			fmt.Printf(" -> OK\n")
		}
	}

	return nil
}

func getSQLText(file string) (string, error) {
	sqlBytes, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	return string(sqlBytes), nil
}

// realScriptExec запускает один скрипт в указанном соединение БД
func realScriptExec(ctx context.Context, db *sql.DB, execTimeout time.Duration, sqlText string) (err error) {
	fail := func(err error) error {
		return err // fmt.Errorf("realScriptExec: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fail(fmt.Errorf("Transaction begin error: %w", err))
	}
	// откат если что сломалось при выполнении
	defer func() {
		if err != nil && tx != nil {
			fmt.Printf(" !! FAIL. Rolling back transaction\n")
			if rerr := tx.Rollback(); rerr != nil {
				err = fail(fmt.Errorf("%v: Transaction rollback error: %w", err, rerr))
			}
		}
	}()

	ctx, cancel := context.WithTimeout(ctx, execTimeout)
	defer cancel()

	if _, err := tx.ExecContext(ctx, sqlText); err != nil {
		return fail(fmt.Errorf("Exec SQL error: %w", err))
	}
	if err = tx.Commit(); err != nil {
		return fail(fmt.Errorf("Transaction сommit error: %w", err))
	}

	return nil
}

func checkDBConnectionSettings(changeset map[DBName]DatabaseChangeset, cfg AndyConfig) []ValidationError {
	var errors []ValidationError
	for dbn := range changeset {
		if _, found := cfg.FindDatabse(dbn); !found {
			errors = append(errors, ValidationError{Object: string(dbn), Message: "Database has no connection settings"})
		}
	}
	return errors
}

func loadConfig(configFile *string) (*AndyConfig, error) {
	if *configFile == "" {
		return nil, fmt.Errorf("Config file path empty, please check --config param value")
	}

	var cfg AndyConfig
	err := cleanenv.ReadConfig(*configFile, &cfg)
	if err != nil {
		return nil, fmt.Errorf("Failed to load config file: %w", err)
	}

	return &cfg, nil
}

type AndyConfig struct {
	Databases    []DatabaseConfig `yaml:"databases"`
	ChangesOrder []string         `yaml:"changes_order" env-default:"procs,funcs,views"`
}

type DatabaseConfig struct {
	DatabaseName         string `yaml:"database" env-default:""`
	Host                 string `yaml:"host" env-default:"localhost"`
	Port                 int    `yaml:"port" env-default:"1433"`
	User                 string `yaml:"user" env-default:"SA"`
	Password             string `yaml:"password"`
	ConnectionTimeoutSec int    `yaml:"connection_timeout_sec" env-default:"30"`
	QueryExecTimeoutSec  int    `yaml:"query_exec_timeout_seс" env-default:"15"`
}

func (ac AndyConfig) FindDatabse(name DBName) (DatabaseConfig, bool) {
	for _, d := range ac.Databases {
		if d.DatabaseName == string(name) {
			return d, true
		}
	}

	return DatabaseConfig{}, false
}

var dbPool map[string]*sql.DB

func makeConnString(dc DatabaseConfig) string {
	query := url.Values{}
	query.Add("database", dc.DatabaseName)
	query.Add("connection timeout", strconv.Itoa(dc.ConnectionTimeoutSec))
	query.Add("app name", fmt.Sprintf("Andy v%s (%s)", buildRelease, vcsRevision))

	u := &url.URL{
		Scheme:   "sqlserver",
		User:     url.UserPassword(dc.User, dc.Password),
		Host:     fmt.Sprintf("%s:%d", dc.Host, dc.Port),
		RawQuery: query.Encode(),
	}
	return u.String()
}

func getDBConnection(ctx context.Context, dc DatabaseConfig) (*sql.DB, error) {
	if db, found := dbPool[dc.DatabaseName]; found {
		return db, nil
	}

	connString := makeConnString(dc)
	db, err := sql.Open("sqlserver", connString)
	if err != nil {
		return nil, fmt.Errorf("Error: connection error: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(dc.ConnectionTimeoutSec)*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("DB connect failed. %w", err)
	}

	return db, nil
}

func readChangedFilesList(input io.Reader) ([]string, error) {
	var files []string
	scanner := bufio.NewScanner(input)

	// Read lines from stdin
	for scanner.Scan() {
		filePath := scanner.Text()

		if path.Ext(filePath) == fileExt {
			files = append(files, filePath)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("Error reading input: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("Nothing to process. Changed files list is empty")
	}

	return files, nil
}

type ScriptInfo struct {
	DatabaseName string
	GroupType    string
	SchemaName   string
	FileName     string
}

func getScriptInfo(file string) (*ScriptInfo, error) {
	dir := path.Dir(file)
	if ok, err := path.Match("*/*/*/*"+fileExt, file); err != nil {
		return nil, fmt.Errorf("File path match error: %w", err)
	} else if !ok {
		return nil, fmt.Errorf("File path invalid format. Expected format: db_name/group_folder/schema/script.sql")
	}
	parts := strings.Split(dir, "/")

	return &ScriptInfo{
		DatabaseName: parts[0],
		GroupType:    parts[1],
		SchemaName:   parts[2],
		FileName:     path.Base(file),
	}, nil
}
