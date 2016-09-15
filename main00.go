package main

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/antonholmquist/jason"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var lastBody []byte
var tenantid string
var userid string
var networkpriid string
var networkpubid string
var roleid map[string]string
var priSubnets []string
var created int
var exists int

type header struct {
	key   string
	value string
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of OpenStack Automator",
	Long:  `All software has versions. This is OpenStack Automator's`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("OpenStack Automator v0.9 -- HEAD")
	},
}

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create new instances based on config file",
	Long:  "",
	Run: func(cmd *cobra.Command, args []string) {
		loadConfig()
		checkConfig()

		fmt.Println("================================================")

		viper.Set("admin.token", getMyToken("admin", viper.GetString("node.admin.password"), "admin"))
		createTenant(viper.GetString("admin.token"))
		createTenantUser(viper.GetString("admin.token"))
		getTenantRole(viper.GetString("admin.token"))
		createTenantUserRoleMap(viper.GetString("admin.token"))
		viper.Set("tenant.token", getMyToken(viper.GetString("create.tenant.username"), viper.GetString("create.tenant.password"), viper.GetString("create.tenant.name")))
		createNetworks(viper.GetString("admin.token"))
		createNetworkRouter(viper.GetString("tenant.token"))
		addNetworkInterfacesToRouter(viper.GetString("admin.token"))
		createFlavours(viper.GetString("tenant.token"))
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete Current instance based on config file",
	Long:  "",
	Run: func(cmd *cobra.Command, args []string) {
		loadConfig()
		checkConfig()

		fmt.Println("================================================")

		viper.Set("admin.token", getMyToken("admin", viper.GetString("node.admin.password"), "admin"))
		fmt.Println(viper.GetString("admin.token"))
		deleteUsers(viper.GetString("admin.token"))
		deleteRouters(viper.GetString("admin.token"))
		deleteNetworks(viper.GetString("admin.token"))
		deleteTenant(viper.GetString("admin.token"))
	},
}

func init() {
	var rootCmd = &cobra.Command{Use: "app"}
	roleid = make(map[string]string)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.Execute()
}

func loadConfig() {
	viper.SetConfigName("config") // name of config file (without extension)
	viper.AddConfigPath(".")      // optionally look for config in the working directory
	err := viper.ReadInConfig()   // Find and read the config file
	if err != nil {               // Handle errors reading the config file
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

func deleteUsers(token string) {
	users, err := getDoesUserExist(viper.GetString("node.url"), viper.GetString("create.tenant.username"), token)
	if err != nil {
		panic(err)
	}
	if users == "" {
		fmt.Println("The user does not exist, nothing required to complete.")
	} else {
		delete, _ := deleteCurrentUser(users, token)
		fmt.Println("User " + viper.GetString("create.tenant.username") + " has been deleted successfully." + delete)
	}
}

func deleteCurrentUser(id string, token string) (string, error) {
	var heads []header
	heads = append(heads, header{"Content-Type", "application/json"})
	heads = append(heads, header{"X-Auth-Token", token})
	buf := ``

	resp, err := getJSONResponse("DELETE", "http://"+viper.GetString("node.url")+":5000/v3/users/"+id, heads, buf)
	if err != nil {
		return "", err
	}
	lastBody = resp

	nid, err := getElementValueString(resp, []string{"router", "id"})
	if err != nil {
		return "", err
	}
	return nid, nil
}

func deleteRouters(token string) {
	router, err := getDoesTenantRouterExistByName(viper.GetString("node.url"), viper.GetString("create.tenant.name")+"-router", token)
	if err != nil {
		panic(err)
	}
	if router == "" {
		fmt.Println("The router does not exist, nothing required to complete.")
	} else {
		fmt.Println("ROUTER:" + router)
		delete, _ := deleteCurrentRouter(router, token)
		fmt.Println("Tenant " + viper.GetString("create.tenant.name") + " has been deleted successfully." + delete)
	}
}

func deleteCurrentRouter(id string, token string) (string, error) {
	var heads []header
	heads = append(heads, header{"Content-Type", "application/json"})
	heads = append(heads, header{"X-Auth-Token", token})
	buf := ``

	resp, err := getJSONResponse("DELETE", "http://"+viper.GetString("node.url")+":9696/v2.0/routers/"+id, heads, buf)
	if err != nil {
		return "", err
	}
	lastBody = resp

	fmt.Println(string(resp))

	nid, err := getElementValueString(resp, []string{"router", "id"})
	if err != nil {
		return "", err
	}
	return nid, nil
}

func deleteNetworks(token string) {
	networks := viper.Get("create.tenant.networks").([]interface{})
	for i, _ := range networks {
		network := networks[i].(map[string]interface{})
		networkName := "-" + network["name"].(string)
		router, _ := getDoesTenantRouterExistByName(viper.GetString("node.url"), viper.GetString("create.tenant.name")+"-router", token)
		subnet, _ := getDoesUserNetworkExist(viper.GetString("node.url"), viper.GetString("create.tenant.name")+networkName, token)
		ports, _ := getDoesTenantPortsExist(viper.GetString("node.url"), viper.GetString("create.tenant.username"), subnet, token)
		fmt.Printf("ROUTER:%s\n", router)
		fmt.Printf("SUBNET:%s\n", subnet)
		fmt.Printf("PORT:%s\n", ports)
		if ports != "" {
			delete, err := deleteCurrentNetworkPort(ports, subnet, router, token)
			fmt.Println("Port " + ports + " has been deleted successfully." + delete)
			fmt.Println(err)
		}
		if subnet == "" {
			fmt.Println("Network " + networkName + " does not exist, nothing to complete.")
		} else {
			delete, _ := deleteCurrentNetwork(subnet, token)
			fmt.Println("Network " + networkName + " has been deleted successfully." + delete)
		}
	}
}

func deleteCurrentNetworkPort(portid string, subnetid string, routerid string, token string) (string, error) {
	var heads []header
	heads = append(heads, header{"Content-Type", "application/json"})
	heads = append(heads, header{"X-Auth-Token", token})
	buf := `{"port_id":"` + portid + `"}`
	//buf := `{"subnet_id":"` + subnetid + `"}`

	fmt.Println("http://" + viper.GetString("node.url") + ":9696/v2.0/routers/" + routerid + "/remove_router_interface")
	fmt.Println(buf)

	resp, err := getJSONResponse("PUT", "http://"+viper.GetString("node.url")+":9696/v2.0/routers/"+routerid+"/remove_router_interface", heads, buf)
	if err != nil {
		return "", err
	}
	lastBody = resp

	fmt.Println(string(resp))

	nid, err := getElementValueString(resp, []string{"id"})
	if err != nil {
		return "", err
	}
	return nid, nil
}

func deleteCurrentNetwork(id string, token string) (string, error) {
	var heads []header
	heads = append(heads, header{"Content-Type", "application/json"})
	heads = append(heads, header{"X-Auth-Token", token})
	buf := ``

	resp, err := getJSONResponse("DELETE", "http://"+viper.GetString("node.url")+":9696/v2.0/networks/"+id, heads, buf)
	if err != nil {
		return "", err
	}
	lastBody = resp

	nid, err := getElementValueString(resp, []string{"router", "id"})
	if err != nil {
		return "", err
	}
	return nid, nil
}

func deleteTenant(token string) {
	tenants, err := getDoesTenantExist(viper.GetString("node.url"), viper.GetString("create.tenant.name"), token)
	if err != nil {
		panic(err)
	}
	if tenants == "" {
		fmt.Println("The tenant does not exist, nothing required to complete.")
	} else {
		delete, _ := deleteCurrentTenant(tenants, token)
		fmt.Println("Tenant " + viper.GetString("create.tenant.name") + " has been deleted successfully." + delete)
	}
}

func deleteCurrentTenant(id string, token string) (string, error) {
	var heads []header
	heads = append(heads, header{"Content-Type", "application/json"})
	heads = append(heads, header{"X-Auth-Token", token})
	buf := ``

	resp, err := getJSONResponse("DELETE", "http://"+viper.GetString("node.url")+":5000/v3/projects/"+id, heads, buf)
	if err != nil {
		return "", err
	}
	lastBody = resp

	nid, err := getElementValueString(resp, []string{"router", "id"})
	if err != nil {
		return "", err
	}
	return nid, nil
}

func createTenant(token string) {
	// Create new tenant/project
	tenants, err := getDoesTenantExist(viper.GetString("node.url"), viper.GetString("create.tenant.name"), token)
	if err != nil {
		panic(err)
	}
	if tenants == "" {
		create, _ := createNewTenant(viper.GetString("node.url"), viper.GetString("create.tenant.name"), token)
		if create != "" {
			fmt.Println("New Tenant " + viper.GetString("create.tenant.name") + " was created successfully, with id: " + create)
		}
		viper.Set("created.tenant.id", create)
		viper.Set("created.tenant.name", viper.GetString("create.tenant.name"))
	} else {
		viper.Set("created.tenant.id", tenants)
		viper.Set("created.tenant.name", viper.GetString("create.tenant.name"))
		fmt.Println("Tenant " + viper.GetString("create.tenant.name") + " already exists with the id: " + viper.GetString("created.tenant.id"))
	}
}

func createTenantUser(token string) {
	// Create new user within project
	users, err := getDoesUserExist(viper.GetString("node.url"), viper.GetString("create.tenant.username"), token)
	if err != nil {
		panic(err)
	}
	if users == "" {
		create, _ := createNewUser(viper.GetString("node.url"), viper.GetString("created.tenant.id"), viper.GetString("create.tenant.username"), viper.GetString("create.tenant.password"), token)
		if create != "" {
			fmt.Println("New User " + viper.GetString("create.tenant.username") + " was created successfully, with id: " + create)
		}
		viper.Set("created.user.id", create)
		viper.Set("created.user.name", viper.GetString("create.tenant.username"))
		viper.Set("created.user.tenant.id", viper.GetString("created.tenant.id"))
	} else {
		viper.Set("created.user.id", users)
		viper.Set("created.user.name", viper.GetString("create.tenant.username"))
		viper.Set("created.user.tenant.id", viper.GetString("created.tenant.id"))
		fmt.Println("User " + viper.GetString("create.tenant.username") + " already exists with the id: " + viper.GetString("created.user.id"))
	}
}

func getTenantRole(token string) {
	// Get roles
	roles, err := getDoesUserRoleExist(viper.GetString("node.url"), viper.GetString("create.tenant.role"), token)
	if err != nil {
		panic(err)
	}
	if roles == "" {
		fmt.Println("Unfortuantely no roles are configured on the system, so we are unable to map a user to a project.  Please correct.")
		return
	}
	roleid[viper.GetString("create.tenant.role")] = roles
	viper.Set("created.user.role", viper.GetString("create.tenant.role"))
	viper.Set("created.user.role.id", roleid[viper.GetString("create.tenant.role")])
	fmt.Println("User role " + viper.GetString("create.tenant.role") + " has been located with id: " + roleid["_member_"])
}

func createTenantUserRoleMap(token string) {
	// Map user to role
	roler, err := createUserProjectRoleRelationship(viper.GetString("node.url"), viper.GetString("created.tenant.id"), viper.GetString("created.user.id"), viper.GetString("created.user.role.id"), token)
	if roler {
		fmt.Println("We have successfully mapped the project, user and role together.")
	} else {
		fmt.Println(err)
	}
}

func createNetworks(token string) {
	networks := viper.Get("create.tenant.networks").([]interface{})
	for i, _ := range networks {
		network := networks[i].(map[string]interface{})
		networkType := network["type"].(string)
		networkName := "-" + network["name"].(string)
		networkExternal := network["external"].(bool)
		subnetName := ""
		subnetDHCP := false
		subnetDNS := "192.168.0.1"
		subnetIPv4 := false
		subnetCIDR := ""
		subnetDHCPStart := ""
		subnetDHCPEnd := ""
		if _, ok := network["dhcp"]; ok {
			subnetDHCP = network["dhcp"].(bool)
		}
		if _, ok := network["dns"]; ok {
			subnetDNS = network["dns"].(string)
		}
		if _, ok := network["ipv4"]; ok {
			subnetIPv4 = network["ipv4"].(bool)
		}
		if _, ok := network["cidr"]; ok {
			subnetCIDR = network["cidr"].(string)
		}
		if _, ok := network["dhcpstart"]; ok {
			subnetDHCPStart = network["dhcpstart"].(string)
		}
		if _, ok := network["dhcpend"]; ok {
			subnetDHCPEnd = network["dhcpend"].(string)
		}

		subnet, err := getDoesUserNetworkExist(viper.GetString("node.url"), viper.GetString("create.tenant.name")+networkName, token)
		if err != nil {
			panic(err)
		}
		if subnet == "" {
			create, _ := createNewUserNetwork(viper.GetString("node.url"), viper.GetString("created.tenant.id"), viper.GetString("create.tenant.name")+networkName, networkExternal, token)
			if create != "" {
				fmt.Println("New network " + viper.GetString("create.tenant.name") + networkName + " was created successfully, with id: " + create)
			}
			viper.Set("created.tenant.network."+viper.GetString("create.tenant.name")+networkName+".name", networkName)
			viper.Set("created.tenant.network."+viper.GetString("create.tenant.name")+networkName+".type", networkType)
			viper.Set("created.tenant.network."+viper.GetString("create.tenant.name")+networkName+".id", create)

		} else {
			viper.Set("created.tenant.network."+viper.GetString("create.tenant.name")+networkName+".name", networkName)
			viper.Set("created.tenant.network."+viper.GetString("create.tenant.name")+networkName+".type", networkType)
			viper.Set("created.tenant.network."+viper.GetString("create.tenant.name")+networkName+".id", subnet)
			fmt.Println("Network " + viper.GetString("create.tenant.name") + " already exists with the id: " + viper.GetString("created.tenant.network."+viper.GetString("create.tenant.name")+networkName+".id"))
		}
		subnetName = viper.GetString("create.tenant.name") + networkName + "-subnet"
		subnet2, _ := getDoesUserSubnetExist(viper.GetString("node.url"), viper.GetString("created.tenant.id"), viper.GetString("created.tenant.network."+viper.GetString("create.tenant.name")+networkName+".id"), subnetName, token)
		if subnet2 == "" {
			create, _ := createNewUserSubnet(viper.GetString("node.url"), viper.GetString("created.tenant.id"), viper.GetString("created.tenant.network."+viper.GetString("create.tenant.name")+networkName+".id"), subnetName, subnetDHCP, subnetDNS, subnetIPv4, subnetCIDR, subnetDHCPStart, subnetDHCPEnd, token)
			if create != "" {
				fmt.Println("New subnet " + viper.GetString("create.tenant.name") + networkName + " was created successfully, with id: " + create)
			}
			if networkType == "private" {
				priSubnets = append(priSubnets, create)
				viper.Set("created.tenant.network."+subnetName+".subnet.id", create)
			} else if networkType == "public" {
				viper.Set("created.tenant.network.public.subnet.name", viper.GetString("create.tenant.name"))
				viper.Set("created.tenant.network.public.subnet.fullname", viper.GetString("create.tenant.name")+viper.GetString("create.tenant.name")+networkName)
				viper.Set("created.tenant.network.public.subnet.id", viper.GetString("created.tenant.network."+viper.GetString("create.tenant.name")+networkName+".id"))
				cidr := strings.Split(subnetCIDR, "/")
				viper.Set("created.tenant.network.public.subnet.mask", createNetworkMask(cidr[1]))
				start := strings.Split(subnetDHCPStart, ".")
				end, _ := strconv.ParseInt(start[3], 10, 64)
				end += 1
				start[3] = strconv.Itoa(int(end))
				viper.Set("created.tenant.network.public.subnet.ip", strings.Join(start, "."))
			}
		} else {
			// TODO:  WE NEED PUBLIC DETAILS IF SUBNET ALREADY EXISTS
			viper.Set("created.tenant.network."+subnetName+".subnet.id", subnet2)
			fmt.Println("Subnet " + subnetName + " already exists with the id: " + viper.GetString("created.tenant.network."+subnetName+".subnet.id"))
		}
	}
}

func createNetworkMask(mask string) string {
	switch mask {
	case "24":
		return "255.255.255.0"
	}
	return ""
}

func getMyToken(username string, password string, tenant string) string {
	token, err := getToken(viper.GetString("node.url"), username, password, tenant)
	if err != nil {
		panic(err)
	}
	return token
}

func addNetworkInterfacesToRouter(token string) {
	//private
	for _, data := range priSubnets {
		data = strings.TrimSpace(data)
		id, err := addRouterInternalNetwork(token, data)
		if id == "" {
			fmt.Println("There was an error: ", err)
		} else {
			fmt.Println("The Internal subnet with the id: " + data + " was successfully added with the port ID: " + id)
		}
	}
	//public
	id, err := addRouterExternalNetwork(token)
	if id == "" {
		fmt.Println("There was an error:", err)
	} else {
		fmt.Println("The External subnet " + viper.GetString("created.tenant.network.public.subnet.fullname") + " was successfully added to " + viper.GetString("created.tenant.network.router.name"))
	}
}

func addRouterInternalNetwork(token string, subnetid string) (string, error) {
	var heads []header
	heads = append(heads, header{"Content-Type", "application/json"})
	heads = append(heads, header{"X-Auth-Token", token})
	buf := `{"subnet_id": "` + subnetid + `"}`
	resp, err := getJSONResponse("PUT", "http://"+viper.GetString("node.url")+":9696/v2.0/routers/"+viper.GetString("created.tenant.network.router.id")+"/add_router_interface", heads, buf)

	if err != nil {
		return "", err
	}
	lastBody = resp

	nid, err := getElementValueString(resp, []string{"port_id"})
	if err != nil {
		return "", err
	}
	return nid, nil
}

func createFlavours(token string) {
	falvours := viper.Get("create.tenant.flavours").([]interface{})
	for i, _ := range falvours {
		flavour := falvours[i].(map[string]interface{})
	}
}

func main() {
}

func createNewUserNetwork(url string, id string, name string, external bool, token string) (string, error) {
	var heads []header
	buf := `{"network": {"name": "` + name + `","admin_state_up": true,"tenant_id":"` + id + `","router:external":` + strconv.FormatBool(external) + `}}`
	heads = append(heads, header{"Content-Type", "application/json"})
	heads = append(heads, header{"X-Auth-Token", token})

	resp, err := getJSONResponse("POST", "http://"+url+":9696/v2.0/networks", heads, buf)
	if err != nil {
		return "", err
	}
	lastBody = resp
	nid, err := getElementValueString(resp, []string{"network", "id"})
	if err != nil {
		return "", err
	}
	return nid, nil
}

func createUserProjectRoleRelationship(url string, tenantid string, userid string, roleid string, token string) (bool, error) {
	var heads []header
	heads = append(heads, header{"Content-Type", "application/json"})
	heads = append(heads, header{"X-Auth-Token", token})
	resp, err := getJSONResponse("PUT", "http://"+url+":5000/v3/projects/"+tenantid+"/users/"+userid+"/roles/"+roleid, heads, "")
	if err != nil {
		return false, err
	}
	lastBody = resp
	return true, nil
}

func createNewUser(url string, id string, username string, password string, token string) (string, error) {
	var heads []header
	buf := `{"user": {"default_project_id": "` + id + `","description": "Created via API tool powered by roporter","domain_id": "default","email": "","enabled": true,"name": "` + username + `","password": "` + password + `"}}`

	heads = append(heads, header{"Content-Type", "application/json"})
	heads = append(heads, header{"X-Auth-Token", token})

	resp, err := getJSONResponse("POST", "http://"+url+":5000/v3/users", heads, buf)
	if err != nil {
		return "", err
	}
	lastBody = resp
	uid, err := getElementValueString(resp, []string{"user", "id"})
	if err != nil {
		return "", err
	}
	return uid, nil
}

func createNewTenant(url string, tenant string, token string) (string, error) {
	var heads []header
	buf := `{"project": {"description": "Created via API tool powered by roporter","domain_id": "default","enabled": true,"is_domain": false,"name": "` + tenant + `"}}`

	heads = append(heads, header{"Content-Type", "application/json"})
	heads = append(heads, header{"X-Auth-Token", token})

	resp, err := getJSONResponse("POST", "http://"+url+":5000/v3/projects", heads, buf)
	if err != nil {
		return "", err
	}
	lastBody = resp
	id, err := getElementValueString(resp, []string{"project", "id"})
	if err != nil {
		return "", err
	}
	return id, nil
}

func getAllUserRolesObject(url string, token string) ([]*jason.Object, error) {
	var heads []header
	heads = append(heads, header{"Content-Type", "application/json"})
	heads = append(heads, header{"X-Auth-Token", token})
	resp, err := getJSONResponse("GET", "http://"+url+":5000/v3/roles", heads, "")
	if err != nil {
		return nil, err
	}
	lastBody = resp
	rroles, err := getElementValueArray(resp, "roles")
	if err != nil {
		return nil, err
	}
	return rroles, errors.New("")
}

func getDoesUserNetworkExist(url string, name string, token string) (string, error) {
	var heads []header
	heads = append(heads, header{"Content-Type", "application/json"})
	heads = append(heads, header{"X-Auth-Token", token})
	resp, err := getJSONResponse("GET", "http://"+url+":9696/v2.0/networks", heads, "")
	if err != nil {
		return "", err
	}
	lastBody = resp
	networks, err := getElementValueArray(resp, "networks")
	if err != nil {
		return "", err
	}
	found := getElementValueWithinObjectArray(networks, "name", name)
	if found != nil {
		tmp, err := found.GetString("id")
		if err != nil {
			return "", err
		}
		return tmp, err
	} else {
		return "", err
	}
}

func getDoesUserRoleExist(url string, role string, token string) (string, error) {
	var heads []header
	heads = append(heads, header{"Content-Type", "application/json"})
	heads = append(heads, header{"X-Auth-Token", token})
	resp, err := getJSONResponse("GET", "http://"+url+":5000/v3/roles", heads, "")
	if err != nil {
		return "", err
	}
	lastBody = resp
	roles, err := getElementValueArray(resp, "roles")
	if err != nil {
		return "", err
	}
	found := getElementValueWithinObjectArray(roles, "name", role)
	if found != nil {
		tmp, err := found.GetString("id")
		if err != nil {
			return "", err
		}
		return tmp, err
	} else {
		return "", err
	}
}

func getDoesTenantExist(url string, tenant string, token string) (string, error) {
	var heads []header
	heads = append(heads, header{"Content-Type", "application/json"})
	heads = append(heads, header{"X-Auth-Token", token})
	resp, err := getJSONResponse("GET", "http://"+url+":5000/v3/projects", heads, "")
	if err != nil {
		return "", err
	}
	lastBody = resp
	tenants, err := getElementValueArray(resp, "projects")
	if err != nil {
		return "", err
	}
	found := getElementValueWithinObjectArray(tenants, "name", tenant)
	if found != nil {
		tmp, err := found.GetString("id")
		if err != nil {
			return "", err
		}
		return tmp, err
	} else {
		return "", err
	}
}

func getDoesUserExist(url string, username string, token string) (string, error) {
	var heads []header
	heads = append(heads, header{"Content-Type", "application/json"})
	heads = append(heads, header{"X-Auth-Token", token})
	resp, err := getJSONResponse("GET", "http://"+url+":5000/v3/users", heads, "")
	if err != nil {
		return "", err
	}
	lastBody = resp
	users, err := getElementValueArray(resp, "users")
	if err != nil {
		return "", err
	}
	found := getElementValueWithinObjectArray(users, "name", username)
	if found != nil {
		tmp, err := found.GetString("id")
		if err != nil {
			return "", err
		}
		return tmp, err
	} else {
		return "", err
	}
}

func getElementValueWithinObjectArray(data []*jason.Object, item string, find string) *jason.Object {
	for _, element := range data {
		res, err := element.GetString(item)
		if err == nil {
			if strings.ToLower(res) == strings.ToLower(find) {
				return element
			}
		}
	}
	return nil
}

func getToken(url string, username string, password string, tenant string) (string, error) {
	var heads []header
	buf := `{"auth":{"passwordCredentials":{"username": "` + username + `", "password": "` + password + `"},"tenantName": "` + tenant + `"}}`
	heads = append(heads, header{"Content-Type", "application/json"})
	resp, err := getJSONResponse("POST", "http://"+url+":5000/v2.0/tokens", heads, buf)
	if err != nil {
		return "", err
	}
	lastBody = resp
	token, err := getElementValueString(resp, []string{"access", "token", "id"})
	if err != nil {
		return "", err
	}
	return token, err
}

func getJSONResponse(method string, url string, headers []header, body string) ([]byte, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		return []byte{}, err
	}
	for i := 0; i < len(headers); i++ {
		head := headers[i]
		req.Header.Add(head.key, head.value)
	}
	resp, err := client.Do(req)
	if err != nil {
		return []byte{}, err
	}
	defer resp.Body.Close()
	rbody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, err
	}
	return rbody, err
}

func getMarshalledResponse(methods string, url string, headers []string, body string) (map[string]interface{}, error) {
	var x map[string]interface{}
	return x, errors.New("")
}

func getElementValueArray(body []byte, find string) ([]*jason.Object, error) {
	v, err := jason.NewObjectFromBytes(body)
	if err != nil {
		return []*jason.Object{}, err
	}
	if find != "" {
		value, err := v.GetObjectArray(find)
		if err != nil {
			return []*jason.Object{}, err
		}
		return value, nil
	}
	return []*jason.Object{}, nil
}

func getElementValueString(body []byte, find []string) (string, error) {
	v, err := jason.NewObjectFromBytes(body)
	if err != nil {
		return "", err
	}
	switch len(find) {
	case 0:
		return "", errors.New("To find value cannot be empty")
	case 1:
		value, err := v.GetString(find[0])
		if err != nil {
			return "", err
		}
		return value, nil
	case 2:
		value, err := v.GetString(find[0], find[1])
		if err != nil {
			return "", err
		}
		return value, nil
	case 3:
		value, err := v.GetString(find[0], find[1], find[2])
		if err != nil {
			return "", err
		}
		return value, nil
	case 4:
		value, err := v.GetString(find[0], find[1], find[2], find[3])
		if err != nil {
			return "", err
		}
		return value, nil
	case 5:
		value, err := v.GetString(find[0], find[1], find[2], find[3], find[4])
		if err != nil {
			return "", err
		}
		return value, nil
	case 6:
		value, err := v.GetString(find[0], find[1], find[2], find[3], find[4], find[5])
		if err != nil {
			return "", err
		}
		return value, nil
	}
	return "", nil
}

func getElementValue2(body []byte, find []string) (string, error) {
	var x map[string]interface{}
	json.Unmarshal(body, &x)
	switch len(find) {
	case 0:
		return "", errors.New("To find value cannot be empty")
	case 1:
		return x[find[0]].(string), nil
	case 2:
		return x[find[0]].(map[string]interface{})[find[1]].(string), nil
	case 3:
		return x[find[0]].(map[string]interface{})[find[1]].(map[string]interface{})[find[2]].(string), nil
	case 4:
		return x[find[0]].(map[string]interface{})[find[1]].(map[string]interface{})[find[2]].(map[string]interface{})[find[3]].(string), nil
	case 5:
		return x[find[0]].(map[string]interface{})[find[1]].(map[string]interface{})[find[2]].(map[string]interface{})[find[3]].(map[string]interface{})[find[4]].(string), nil
	case 6:
		return x[find[0]].(map[string]interface{})[find[1]].(map[string]interface{})[find[2]].(map[string]interface{})[find[3]].(map[string]interface{})[find[4]].(map[string]interface{})[find[5]].(string), nil
	}
	return "", errors.New("An unknown error has occured")
}

func getDoesUserSubnetExist(url string, tenantid string, networkid string, subnetname string, token string) (string, error) {
	var heads []header
	heads = append(heads, header{"Content-Type", "application/json"})
	heads = append(heads, header{"X-Auth-Token", token})
	resp, err := getJSONResponse("GET", "http://"+url+":9696/v2.0/subnets", heads, "")
	if err != nil {
		return "", err
	}
	lastBody = resp
	subnets, err := getElementValueArray(resp, "subnets")
	if err != nil {
		return "", err
	}
	found := getElementValueWithinObjectArray(subnets, "name", subnetname)
	if found != nil {
		id1, err := found.GetString("tenant_id")
		id2, err := found.GetString("network_id")
		if strings.TrimSpace(id1) == tenantid && strings.TrimSpace(id2) == networkid {
			tmp, err := found.GetString("id")
			if err != nil {
				return "", err
			}
			return tmp, err
		}
		return "", err
	} else {
		return "", err
	}
}

func createNewUserSubnet(url string, tenantid string, networkid string, subnetname string, subnetDHCP bool, subnetDNS string, subnetIPv4 bool, subnetCIDR string, subnetDHCPStart string, subnetDHCPEnd string, token string) (string, error) {
	var heads []header
	buf := `{
	    "subnet": {
	        "name": "` + subnetname + `",
	        "enable_dhcp": ` + strconv.FormatBool(subnetDHCP) + `,
	        "network_id": "` + networkid + `",
	        "tenant_id": "` + tenantid + `",
	        "dns_nameservers": ["` + subnetDNS + `"],
	        "allocation_pools": [
	            {
	                "start": "` + subnetDHCPStart + `",
	                "end": "` + subnetDHCPEnd + `"
	            }
	        ],
	        "host_routes": [],
	        "ip_version": 4,
	        "cidr": "` + subnetCIDR + `"
	    }
	}`
	heads = append(heads, header{"Content-Type", "application/json"})
	heads = append(heads, header{"X-Auth-Token", token})

	resp, err := getJSONResponse("POST", "http://"+url+":9696/v2.0/subnets", heads, buf)

	if err != nil {
		return "", err
	}
	lastBody = resp

	nid, err := getElementValueString(resp, []string{"subnet", "id"})
	if err != nil {
		return "", err
	}
	return nid, nil
}

func getDoesTenantRouterExistByName(url string, name string, token string) (string, error) {
	var heads []header
	heads = append(heads, header{"Content-Type", "application/json"})
	heads = append(heads, header{"X-Auth-Token", token})
	resp, err := getJSONResponse("GET", "http://"+url+":9696/v2.0/routers", heads, "")
	if err != nil {
		return "", err
	}
	lastBody = resp
	routers, err := getElementValueArray(resp, "routers")
	if err != nil {
		return "", err
	}
	found := getElementValueWithinObjectArray(routers, "name", name)
	if found != nil {
		tmp, err := found.GetString("id")
		if err != nil {
			return "", err
		}
		return tmp, err
	} else {
		return "", err
	}
}

func getDoesTenantRouterExist(url string, id string, token string) (string, error) {
	var heads []header
	heads = append(heads, header{"Content-Type", "application/json"})
	heads = append(heads, header{"X-Auth-Token", token})
	resp, err := getJSONResponse("GET", "http://"+url+":9696/v2.0/routers", heads, "")
	if err != nil {
		return "", err
	}
	lastBody = resp
	routers, err := getElementValueArray(resp, "routers")
	if err != nil {
		return "", err
	}
	found := getElementValueWithinObjectArray(routers, "tenant_id", id)
	if found != nil {
		tmp, err := found.GetString("id")
		if err != nil {
			return "", err
		}
		return tmp, err
	} else {
		return "", err
	}
}

func getDoesTenantPortsExist(url string, name string, id string, token string) (string, error) {
	var heads []header
	heads = append(heads, header{"Content-Type", "application/json"})
	heads = append(heads, header{"X-Auth-Token", token})
	resp, err := getJSONResponse("GET", "http://"+url+":9696/v2.0/ports", heads, "")
	if err != nil {
		return "", err
	}
	lastBody = resp
	ports, err := jason.NewObjectFromBytes(resp)
	p, err := ports.GetObjectArray("ports")

	found, err := getElementValueForPortExists(p, id)

	if found != "" {
		fmt.Printf("FOUND PORT ID:%s\n", found)
		return found, nil
	}
	return "", err
}

func getElementValueForPortExists(p []*jason.Object, id string) (string, error) {
	for _, element := range p {
		tmp2, _ := element.GetObjectArray("fixed_ips")
		tmp3, _ := tmp2[0].GetString("subnet_id")
		fmt.Printf("FOUND:%s\nLOOKING FOR:%s\n", tmp3, id)
		if strings.TrimSpace(tmp3) == id {
			fmt.Println(tmp3)
			tmp, err := element.GetString("id")
			if err == nil {
				fmt.Printf("NETWORK ID: " + id + "\n ID: " + tmp + "\n")
				return strings.TrimSpace(tmp), nil
			}
		}
	}
	return "", nil
}

func createNetworkRouter(token string) {
	// Get routers
	routers, err := getDoesTenantRouterExist(viper.GetString("node.url"), viper.GetString("created.tenant.id"), token)
	if err != nil {
		panic(err)
	}
	name := viper.GetString("created.tenant.name") + "-router"
	if routers == "" {
		create, _ := createNewTenantRouter(viper.GetString("node.url"), viper.GetString("created.tenant.id"), token)
		if create != "" {
			fmt.Println("New router " + name + " successfully created with the id: " + create)
		}
		viper.Set("created.tenant.network.router.id", create)
		viper.Set("created.tenant.network.router.name", name)
	} else {
		viper.Set("created.tenant.network.router.id", routers)
		viper.Set("created.tenant.network.router.name", name)
		fmt.Println("Router " + viper.GetString("created.tenant.network.router.name") + " already exists with the id: " + viper.GetString("created.tenant.network.router.id"))
	}
}

func createNewTenantRouter(url string, tenantid string, token string) (string, error) {
	var heads []header
	heads = append(heads, header{"Content-Type", "application/json"})
	heads = append(heads, header{"X-Auth-Token", token})
	buf := `{
    "router": {
        "name": "` + viper.GetString("created.tenant.network.public.subnet.name") + `-router",
        "admin_state_up": true
    }
	}`
	resp, err := getJSONResponse("POST", "http://"+url+":9696/v2.0/routers", heads, buf)
	if err != nil {
		return "", err
	}
	lastBody = resp

	nid, err := getElementValueString(resp, []string{"router", "id"})
	if err != nil {
		return "", err
	}
	return nid, nil
}

func addRouterExternalNetwork(token string) (string, error) {
	var heads []header
	heads = append(heads, header{"Content-Type", "application/json"})
	heads = append(heads, header{"X-Auth-Token", token})
	buf := `{
	    "router": {
	        "external_gateway_info": {
	            "network_id": "` + viper.GetString("created.tenant.network.public.subnet.id") + `"
			}
		}
	}`

	resp, err := getJSONResponse("PUT", "http://"+viper.GetString("node.url")+":9696/v2.0/routers/"+viper.GetString("created.tenant.network.router.id"), heads, buf)
	if err != nil {
		return "", err
	}
	lastBody = resp

	nid, err := getElementValueString(resp, []string{"router", "id"})
	if err != nil {
		return "", err
	}
	return nid, nil
}
