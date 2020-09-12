package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

var tableFlag string
var fileName string
var filePath string
var fileType string
var configFile string
var dbUsername string
var dbPassword string
var dbDatabase string
var dbHostname string
var dbPortNumber string
var backupDate string
var bashCommand string
var importFile string
var includeDate bool

type LaravelEnvFile map[string]string

/**
read value from env file
*/
func ReadPropertiesFile(filename string) (LaravelEnvFile, error) {

	config := LaravelEnvFile{}

	if len(filename) == 0 {
		return config, nil
	}
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if equal := strings.Index(line, "="); equal >= 0 {
			if key := strings.TrimSpace(line[:equal]); len(key) > 0 {
				value := ""
				if len(line) > equal {
					value = strings.TrimSpace(line[equal+1:])
				}
				config[key] = value
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
		return nil, err
	}

	return config, nil
}

/**
check if array contains an string value or not
*/
func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// getValidFileTypes returns an array of acceptable file extensions
func getValidFileTypes() []string {
	return []string{"sql", "zip", "gz"}
}

/**
get the output file path
*/
func getOutputFilePath() string {
	if includeDate {
		return filePath + fileName + backupDate
	} else {
		return filePath + fileName
	}
}

/**
get the sql output file path
*/
func getSqlOutput() string {
	return getOutputFilePath() + ".sql"
}

/**
parse input flags
*/
func parseFlags() {
	flag.StringVar(&tableFlag, "t", "*", "name of table(s) you want to dump comma separated.")
	flag.StringVar(&fileName, "n", "dump", "output file name")
	flag.StringVar(&filePath, "p", "", "output file path")
	flag.StringVar(&fileType, "f", "sql", "output file type (sql|zip|gz)")
	flag.BoolVar(&includeDate, "d", false, "add date to output files")
	flag.StringVar(&importFile, "i", "", "import file path")

	flag.StringVar(&dbUsername, "db_user", "", "database user name")
	flag.StringVar(&dbPassword, "db_pass", "", "database user password")
	flag.StringVar(&dbHostname, "db_host", "", "database host")
	flag.StringVar(&dbDatabase, "db_name", "", "database name")
	flag.StringVar(&dbPortNumber, "db_port", "3306", "database port number")

	flag.StringVar(&configFile, "config", "", "database config file")

	flag.Parse()

	if !contains(getValidFileTypes(), fileType) {
		console("invalid file type ")
	}
}

func getTableNames() string {
	if tableFlag == "*" {
		return ""
	}
	return strings.Replace(tableFlag, ",", " ", 3)
}

/**
create bash command and store it
*/
func runDumpCommand() {
	console("dumping database...")

	bashCommand = "mysqldump -h" +
		dbHostname +
		" -P" + dbPortNumber +
		" -u " + dbUsername +
		" -p" + dbPassword + " " +
		dbDatabase + " " +
		getTableNames() + "  > " +
		getSqlOutput()

	_, err := exec.Command("sh", "-c", bashCommand).Output()
	if err != nil {
		console("Error! Export Failed!")
		removeMainFile()
		os.Exit(0)
	}
}

/**
create bash command and store it
*/
func runImportCommand() {
	console("importing database...")
	extractImportFile()
	bashCommand = "mysql -h" +
		dbHostname +
		" -P" + dbPortNumber +
		" -u " + dbUsername +
		" -p" + dbPassword + " " +
		dbDatabase + " < " +
		importFile

	executeCommand(bashCommand, "Import Failed!")

	// remove the tmp file if exists
	os.Remove("tmp.sql")
}

/**
extract the import file if it is gz or zip
*/
func extractImportFile() {
	contentType := getFileContentType()
	if contentType == "application/x-gzip" || contentType == "application/zip" {
		bashCommand = "gunzip -c " + importFile + " > tmp.sql"
		executeCommand(bashCommand, "Cannot decompress the file")
		importFile = "tmp.sql"
	}
}

/**
get importFile content type
*/
func getFileContentType() string {
	f, err := os.Open(importFile)
	if err != nil {
		console("Error! " + importFile + " not found!")
		os.Exit(0)
	}
	defer f.Close()
	// Only the first 512 bytes are used to sniff the content type.
	buffer := make([]byte, 512)

	_, err = f.Read(buffer)
	if err != nil {
		console("Error! Cannot read " + importFile)
		os.Exit(0)
	}

	// Use the net/http package's handy DectectContentType function. Always returns a valid
	// content-type by returning "application/octet-stream" if no others seemed to match.
	contentType := http.DetectContentType(buffer)

	return contentType
}

/**
compress output file
*/
func runCompressCommand() {
	console("compressing dump file... ")
	if fileType == "zip" {
		bashCommand = "zip " + getOutputFilePath() + ".zip " + getSqlOutput()
		executeCommand(bashCommand, "Cannot compress file.")
	} else if fileType == "gz" {
		bashCommand = "gzip -c " + getSqlOutput() + " > " + getOutputFilePath() + ".gz"
		executeCommand(bashCommand, "Cannot compress file.")
	}

	// remove .sql file
	if fileType != "sql" {
		removeMainFile()
	}
}

/**
remove main file
*/
func removeMainFile() {
	err := os.Remove(getSqlOutput())
	if err != nil {
		console("Error! Cannot Delete .sql file")
		os.Exit(0)
	}
}

/**
Execute the command and console the error if it has
*/
func executeCommand(bashCommand string, message string) {
	_, err := exec.Command("sh", "-c", bashCommand).Output()
	if err != nil {
		console("Error! " + message)
		os.Exit(0)
	}
}

/**
read environment file
*/
func readEnv() {
	var config, _ = ReadPropertiesFile(configFile)
	dbDatabase = config["DB_DATABASE"]
	dbHostname = config["DB_HOST"]
	dbUsername = config["DB_USERNAME"]
	dbPassword = config["DB_PASSWORD"]
	dbPortNumber = config["DB_PORT"]
}

/**
get iso Date & time
*/
func isoDate(kebabcase bool) string {
	dt := time.Now()
	if kebabcase {
		var kc = strings.Replace(dt.Format("01-02-2006 15:04:05"), " ", "-", -1)
		kc = strings.Replace(kc, ":", "-", -1)
		return kc
	}
	return dt.Format("01-02-2006 15:04:05")

}

/**
console output
*/
func console(message string) {
	fmt.Println(isoDate(false) + " :: " + message)
}

func main() {
	backupDate = isoDate(true)
	parseFlags()
	if configFile != "" {
		readEnv()
	}

	if importFile != "" {
		runImportCommand()
	} else {
		runDumpCommand()
		if fileType == "zip" || fileType == "gz" {
			runCompressCommand()
		}
	}

	console("done!")
}
