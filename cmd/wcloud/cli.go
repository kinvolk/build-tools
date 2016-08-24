package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
	"gopkg.in/yaml.v2"
)

func env(key, def string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return def
}

func instancesFile() string {
	u, err := user.Current()
	if err != nil {
		panic(err)
	}

	return filepath.Join(u.HomeDir, ".wcloudconfig")
}

func usage() {
	fmt.Printf(`Usage: wcloud COMMAND ...
	deploy <image>:<version>   Deploy image to your configured env
	list                       List recent deployments
	config (<filename>)        Get (or set) the configured env
	logs <deploy>              Show lots for the given deployment
	instance (<name>)          Get (or set) the configured instance

	Environment Variables:
	  SERVICE_TOKEN            Set the service token to use, overrides %s
	  BASE_URL                 Set the deploy to connect to, overrides %s
`,
		instancesFile(),
		instancesFile())
}

func main() {
	if len(os.Args) <= 1 {
		usage()
		os.Exit(1)
	}

	currentInstance, err := loadCurrentInstance()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	token := env("SERVICE_TOKEN", currentInstance.ServiceToken)
	baseURL := env("BASE_URL", currentInstance.BaseURL)
	if baseURL == "" {
		baseURL = "https://cloud.weave.works"
	}

	c := NewClient(token, baseURL)

	switch os.Args[1] {
	case "deploy":
		deploy(c, os.Args[2:])
	case "list":
		list(c, os.Args[2:])
	case "config":
		config(c, os.Args[2:])
	case "instance":
		instance(c, os.Args[2:])
	case "logs":
		logs(c, os.Args[2:])
	case "events":
		events(c, os.Args[2:])
	case "help":
		usage()
	default:
		usage()
	}
}

func newFlagSet() *flag.FlagSet {
	flags := flag.NewFlagSet("", flag.ContinueOnError)
	flags.Usage = usage
	return flags
}

func deploy(c Client, args []string) {
	var (
		flags    = newFlagSet()
		username = flags.String("u", "", "Username to report to deploy service (default with be current user)")
	)
	if err := flags.Parse(args); err != nil {
		usage()
		return
	}
	args = flags.Args()
	if len(args) != 1 {
		usage()
		return
	}
	parts := strings.SplitN(args[0], ":", 2)
	if len(parts) < 2 {
		usage()
		return
	}
	if *username == "" {
		user, err := user.Current()
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		*username = user.Username
	}
	deployment := Deployment{
		ImageName:      parts[0],
		Version:        parts[1],
		TriggeringUser: *username,
	}
	if err := c.Deploy(deployment); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func list(c Client, args []string) {
	var (
		flags = newFlagSet()
		since = flags.Duration("since", 7*24*time.Hour, "How far back to fetch results")
	)
	if err := flags.Parse(args); err != nil {
		usage()
		return
	}
	through := time.Now()
	from := through.Add(-*since)
	deployments, err := c.GetDeployments(from.Unix(), through.Unix())
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Created", "ID", "Image", "Version", "State"})
	table.SetBorder(false)
	table.SetColumnSeparator(" ")
	for _, deployment := range deployments {
		table.Append([]string{
			deployment.CreatedAt.Format(time.RFC822),
			deployment.ID,
			deployment.ImageName,
			deployment.Version,
			deployment.State,
		})
	}
	table.Render()
}

func events(c Client, args []string) {
	var (
		flags = newFlagSet()
		since = flags.Duration("since", 7*24*time.Hour, "How far back to fetch results")
	)
	if err := flags.Parse(args); err != nil {
		usage()
		return
	}
	through := time.Now()
	from := through.Add(-*since)
	events, err := c.GetEvents(from.Unix(), through.Unix())
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	fmt.Println("events: ", string(events))
}

func loadConfig(filename string) (*Config, error) {
	extension := filepath.Ext(filename)
	var config Config
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	if extension == ".yaml" || extension == ".yml" {
		if err := yaml.Unmarshal(buf, &config); err != nil {
			return nil, err
		}
	} else {
		if err := json.NewDecoder(bytes.NewReader(buf)).Decode(&config); err != nil {
			return nil, err
		}
	}
	return &config, err
}

func config(c Client, args []string) {
	if len(args) > 1 {
		usage()
		return
	}

	if len(args) == 1 {
		config, err := loadConfig(args[0])
		if err != nil {
			fmt.Println("Error reading config:", err)
			os.Exit(1)
		}

		if err := c.SetConfig(config); err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
	} else {
		config, err := c.GetConfig()
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		buf, err := yaml.Marshal(config)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		fmt.Println(string(buf))
	}
}

func loadInstances() (InstancesFile, error) {
	filename := instancesFile()
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return InstancesFile{}, nil
		}
		return InstancesFile{}, err
	}
	var instances InstancesFile
	if err := yaml.Unmarshal(buf, &instances); err != nil {
		return InstancesFile{}, err
	}
	return instances, err
}

func loadCurrentInstance() (*Instance, error) {
	instances, err := loadInstances()
	if err != nil {
		return nil, err
	}
	return instances.Instances[instances.Current], nil
}

func saveInstances(i InstancesFile) error {
	buf, err := yaml.Marshal(i)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(instancesFile(), buf, 0600)
}

func instance(c Client, args []string) {
	if len(args) > 1 {
		usage()
		return
	}

	if len(args) == 1 {
		// Setting the current instance name
		instances, err := loadInstances()
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		_, ok := instances.Instances[args[0]]
		if !ok {
			fmt.Printf("Instance %q not found in %s\n", args[0], instancesFile())
			fmt.Printf("Available: %s\n", strings.Join(instances.Available(), " "))
			os.Exit(1)
		}

		instances.Current = args[0]
		if err := saveInstances(instances); err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
	} else {
		// Getting the current instance
		instances, err := loadInstances()
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		if instances.Current == "" {
			fmt.Println("<none>")
			fmt.Printf("Available: %s\n", strings.Join(instances.Available(), " "))
			os.Exit(0)
		}

		current, ok := instances.Instances[instances.Current]
		if !ok {
			fmt.Printf("Instance %q not found in %s\n", args[0], instancesFile())
			os.Exit(1)
		}

		buf, err := yaml.Marshal(map[string]*Instance{
			instances.Current: current,
		})
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		fmt.Println(string(buf))
	}
}

func logs(c Client, args []string) {
	if len(args) != 1 {
		usage()
		return
	}

	output, err := c.GetLogs(args[0])
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	fmt.Println(string(output))
}
