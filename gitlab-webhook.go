package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

//GitlabRepository represents repository information from the webhook
type GitlabRepository struct {
	Name        string
	URL         string
	Description string
	Home        string
}

//Commit represents commit information from the webhook
type Commit struct {
	ID        string
	Message   string
	Timestamp string
	URL       string
	Author    Author
}

//Author represents author information from the webhook
type Author struct {
	Name  string
	Email string
}

//Webhook represents push information from the webhook
type Webhook struct {
	Before            string
	After             string
	Ref               string
	Username          string
	UserID            int
	ProjectID         int
	Repository        GitlabRepository
	Commits           []Commit
	TotalCommitsCount int
}

//ConfigRepository represents a repository from the config file
type ConfigRepository struct {
	Name     string
	Commands []string
}

//Config represents the config file
type Config struct {
	Address      string
	Port         int64
	Repositories []ConfigRepository
}

func PanicIf(err error, what ...string) {
	if err != nil {
		if len(what) == 0 {
			panic(err)
		}

		panic(errors.New(err.Error() + what[0]))
	}
}

var (
	cfg Config
)

func main() {
	var config Config
	var configFile string

	flag.StringVar(&configFile, "config", "config.json", "configuration file to load")

	flag.Parse()

	if len(flag.Args()) != 0 {
		log.Fatal("extra arguments provided")
	}

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGHUP)

	// reload configuration on SIGHUP
	go func() {
		<-sigc
		var err error
		config, err = loadConfig(configFile)
		if err != nil {
			log.Fatalf("Failed to read config: %s", err)
		}
		log.Println("config reloaded")
	}()

	//load config
	var err error
	cfg, err = loadConfig(configFile)
	if err != nil {
		log.Fatalf("Failed to read config: %s", err)
	}

	// setting handler
	http.HandleFunc("/", hookHandler)

	address := fmt.Sprintf("%s:%d", cfg.Address, cfg.Port)

	log.Println("Listening on " + address)

	// starting server
	err = http.ListenAndServe(address, nil)
	if err != nil {
		log.Println(err)
	}
}

func loadConfig(configFile string) (Config, error) {
	var cfg Config
	file, err := os.Open(configFile)
	if err != nil {
		return Config{}, err
	}
	defer file.Close()

	buffer := make([]byte, 1024)
	count := 0

	count, err = file.Read(buffer)
	if err != nil {
		return Config{}, err
	}

	err = json.Unmarshal(buffer[:count], &cfg)
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func hookHandler(w http.ResponseWriter, r *http.Request) {
	var hook Webhook

	//read request body
	var data, err = ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("Failed to read request: %s", err)
		return
	}

	//unmarshal request body
	err = json.Unmarshal(data, &hook)
	if err != nil {
		log.Printf("Failed to parse request: %s", err)
		return
	}

	//find matching config for repository name
	for _, repo := range cfg.Repositories {
		if repo.Name != hook.Repository.Name {
			continue
		}

		//execute commands for repository
		for _, cmd := range repo.Commands {
			var command = exec.Command(cmd)
			out, err := command.Output()
			if err != nil {
				log.Printf("Failed to execute command: %s", err)
				continue
			}
			log.Println("Executed: " + cmd)
			log.Println("Output: " + string(out))
		}
	}
}
