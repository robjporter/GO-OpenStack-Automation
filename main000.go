package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

const OS_Neutron_Port = ":9696/v2.0/"
const OS_Keystone_Port = ":5000/v3/"
const loginJSON = `{"auth":{"passwordCredentials":{"username": "<USERNAME>", "password": "<PASSWORD>"},"tenantName": "<TENANT>"}}`
const createTenantJSON = `{"project": {"description": "Created via API tool powered by roporter","domain_id": "default","enabled": true,"is_domain": false,"name": "<TENANT>"}}`
const createTenantUser = `{"user": {"default_project_id": "<ID>","description": "Created via API tool powered by roporter","domain_id": "default","email": "","enabled": true,"name": "<USERNAME>","password": "<PASSWORD>"}}`

var configName string
var doCommands map[string]commands
var undoCommands map[string]commands
var urls map[string]string
var defaultHeaders map[string][]headers

type headers struct {
	key   string
	value string
}
type commands struct {
	url      string    // HTTP URL
	method   string    // HTTP METHOD - POST/GET/PUT
	heads    []headers // HTTP Headers
	body     string    // HTTP Payload
	name     string    // the viper key or key to store result in
	requires []string  // the viper key or keys from a key/value store needed
}
type replacements struct {
	key   string
	value string
}

func init() {
	configName = "config"
	doCommands = make(map[string]commands)
	undoCommands = make(map[string]commands)
	urls = make(map[string]string)
	defaultHeaders = make(map[string][]headers)
}

func main() {
	loadConfig()
	checkConfig()
	buildURLs()
	fmt.Println("================================================")
	buildCommandSets() // GET ADMIN TOKEN
	executeDoCommands()
	fmt.Println("================================================")
	fmt.Println(doCommands)
	fmt.Println(defaultHeaders)
}

// DO COMMANDS
func executeDoCommands() {
	buildDefaultHeaders()
	buildDefaultAdminHeaders()
	buildDefaultUserHeaders()
	for i, data := range doCommands {
		fmt.Printf("%s:%s\n", i, data)
	}
	//emptyDoCommands()
	doCommands = make(map[string]commands)
}

// HEADERS
func buildDefaultHeaders() {
	defaultHeaders["default"] = append(defaultHeaders["default"], headers{"Content-Type", "application/json"})
	defaultHeaders["admin"] = defaultHeaders["default"]
	defaultHeaders["user"] = defaultHeaders["default"]
}
func buildDefaultAdminHeaders() {
	defaultHeaders["admin"] = append(defaultHeaders["admin"], headers{"X-Auth-Token", viper.GetString("tokens.admin")})
}
func buildDefaultUserHeaders() {
	defaultHeaders["user"] = append(defaultHeaders["user"], headers{"X-Auth-Token", viper.GetString("tokens.user")})
}

// TOKEN FUNCTIONS
func getToken(url string, username string, password string, tenant string, result string) {
	tmp := commands{}
	tmp.body = replaceValues(loginJSON, []replacements{replacements{key: "<USERNAME>", value: username}, replacements{key: "<PASSWORD>", value: password}, replacements{key: "<TENANT>", value: tenant}})
	tmp.heads = defaultHeaders["default"]
	tmp.method = "POST"
	tmp.name = result
	tmp.url = urls["keystone"] + "tokens"
	tmp.requires = nil
	doCommands["GetAdminToken"] = tmp
	undoCommands["GetAdminToken"] = tmp
}

// CONFIG FUNCTIONS
func loadConfig() {
	viper.SetConfigName(configName) // name of config file (without extension)
	viper.AddConfigPath(".")        // optionally look for config in the working directory
	err := viper.ReadInConfig()     // Find and read the config file
	if err != nil {                 // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
}
func checkConfig() {
	reader := bufio.NewReader(os.Stdin)
	if viper.GetString("node.url") == "" {
		fmt.Print("Enter url of OpenStack Instance: ")
		url, _ := reader.ReadString('\n')
		viper.Set("node.url", strings.TrimSpace(url))
	}
	if viper.GetString("node.admin.password") == "" {
		fmt.Print("Enter Admin Password: ")
		password, _ := reader.ReadString('\n')
		viper.Set("node.admin.password", strings.TrimSpace(password))
	}
	if viper.GetString("create.tenant.name") == "" {
		fmt.Print("Enter new Tenant Name: ")
		tenant, _ := reader.ReadString('\n')
		viper.Set("create.tenant.name", strings.TrimSpace(tenant))
	}
	if viper.GetString("create.tenant.username") == "" {
		fmt.Print("Enter new Tenant username: ")
		tenantuser, _ := reader.ReadString('\n')
		viper.Set("create.tenant.username", strings.TrimSpace(tenantuser))
	}
	if viper.GetString("create.tenant.password") == "" {
		fmt.Print("Enter new Tenant user password: ")
		tenantuserpassword, _ := reader.ReadString('\n')
		viper.Set("create.tenant.password", strings.TrimSpace(tenantuserpassword))
	}
	if viper.GetString("create.tenant.role") == "" {
		fmt.Print("Enter new Tenant user role: ")
		tenantuserrole, _ := reader.ReadString('\n')
		viper.Set("create.tenant.role", strings.TrimSpace(tenantuserrole))
	}
	if viper.GetString("create.tenant.ip") == "" {
		fmt.Print("Enter new Tenant Private IP Space: (ip/mask) ")
		tenantiprange, _ := reader.ReadString('\n')
		viper.Set("create.tenant.ip", strings.TrimSpace(tenantiprange))
	}
}

// INITIALISE CONFIG FUNCTIONS
func buildURLs() {
	urls["base"] = "http://" + viper.GetString("node.url")
	urls["neutron"] = urls["base"] + OS_Neutron_Port
	urls["keystone"] = urls["base"] + OS_Keystone_Port
}
func buildCommandSets() {
	fmt.Printf("Base:    %s\n", urls["base"])
	fmt.Printf("Neutron: %s\n", urls["neutron"])
	fmt.Printf("Keystone:%s\n", urls["keystone"])
	getToken(viper.GetString("node.url"), "admin", viper.GetString("node.admin.password"), "admin", "tokens.admin")
	getCreateTenantWithAdminUser(viper.GetString("node.url"), viper.GetString("create.tenant.name"), "created.tenant.id")
	getCreateTenantUserWithAdminUser(viper.GetString("node.url"), viper.GetString("create.tenant.name"), viper.GetString("create.tenant.username"), viper.GetString("create.tenant.password"), "created.tenant.user.id")
	getToken(viper.GetString("node.url"), viper.GetString("create.tenant.username"), viper.GetString("create.tenant.password"), viper.GetString("create.tenant.name"), "tokens.user")
}

// TENANT FUNCTIONS
func getCreateTenantWithAdminUser(url string, tenant string, result string) {
	tmp := commands{}
	tmp.body = replaceValues(createTenantJSON, []replacements{replacements{key: "<TENANT>", value: tenant}})
	tmp.heads = defaultHeaders["admin"]
	tmp.method = "POST"
	tmp.name = result
	tmp.url = urls["keystone"] + "projects"
	tmp.requires = nil
	doCommands["CreateNewTenant"] = tmp
	undoCommands["CreateNewTenant"] = tmp
}
func getCreateTenantUserWithAdminUser(url string, tenant string, username string, password string, result string) {
	tmp := commands{}
	tmp.body = replaceValues(createTenantJSON, []replacements{replacements{key: "<ID>", value: viper.GetString("created.tenant.id")}, replacements{key: "<USERNAME>", value: username}, replacements{key: "<PASSWORD>", value: password}})
	tmp.heads = defaultHeaders["admin"]
	tmp.method = "POST"
	tmp.name = result
	tmp.url = urls["keystone"] + "users"
	tmp.requires = nil
	doCommands["CreateNewTenantUser"] = tmp
	undoCommands["CreateNewTenantUser"] = tmp
}

// GENERAL FUNCTIONS
func createNetworkMask(mask string) string {
	switch mask {
	case "8":
		return "255.0.0.0"
	case "16":
		return "255.255.0.0"
	case "24":
		return "255.255.255.0"
	case "30":
		return "255.255.255.252"
	}
	return ""
}
func replaceValues(text string, replace []replacements) string {
	for _, data := range replace {
		text = strings.Replace(text, strings.TrimSpace(data.key), strings.TrimSpace(data.value), -1)
	}
	return text
}
