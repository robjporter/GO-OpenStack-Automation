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
	"strings"
)

func main() {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter url of OpenStack Instance: ")
	url, _ := reader.ReadString('\n')
	url = strings.TrimSpace(url)
	fmt.Print("Enter Admin Password: ")
	password, _ := reader.ReadString('\n')
	password = strings.TrimSpace(password)

	buf := `{"auth":{"passwordCredentials":{"username": "admin", "password": "` + password + `"},"tenantName": "admin"}}`
	resp, err := client.Post("http://"+url+":5000/v2.0/tokens", "application/json", strings.NewReader(buf))
	if err != nil {
		return
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	var x map[string]interface{}
	json.Unmarshal(body, &x)
	token := x["access"].(map[string]interface{})["token"].(map[string]interface{})["id"]
	fmt.Printf("TOKEN: %v\n", token)

	req, err := http.NewRequest("GET", "http://"+url+":5000/v2.0/tenants", strings.NewReader(""))
	if err != nil {
		return
	}
	req.Header.Add("X-Auth-Token", token.(string))
	req.Header.Add("Content-Type", "application/json")
	resp, err = client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	fmt.Println(string(body))
	var y map[string]interface{}
	json.Unmarshal(body, &y)
	fmt.Printf("TENANT RESPONSE: %s\n", y)

}

func getJSONResponse(headers []string) (string, error) {
	return "", errors.New("")
}

func getMarshalledResponse(headers []string) (map[string]interface{}, error) {
	var x map[string]interface{}
	return x, errors.New("")
}
