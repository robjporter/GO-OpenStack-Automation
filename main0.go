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

var lastBody []byte

type header struct {
	key   string
	value string
}

func main() {

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter url of OpenStack Instance: ")
	url, _ := reader.ReadString('\n')
	url = strings.TrimSpace(url)
	fmt.Print("Enter Admin Password: ")
	password, _ := reader.ReadString('\n')
	password = strings.TrimSpace(password)

	token, err := getToken(url, password)
	if err != nil {
		return
	}

	users, err := getDoesUserExist(url, "roporter", token)
	fmt.Println(users)
	fmt.Println(err)
}

func getDoesUserExist(url string, username string, token string) (bool, error) {
	var heads []header
	heads = append(heads, header{"Content-Type", "application/json"})
	heads = append(heads, header{"X-Auth-Token", token})
	resp, err := getJSONResponse("GET", "http://"+url+":5000/v3/users", heads, "")
	if err != nil {
		return false, err
	}
	lastBody = resp
	users, err := getElementValue(resp, []string{"users[0]", "name"})
	if err != nil {
		return false, err
	}
	fmt.Println(string(users))
	return true, err
}

func getToken(url string, password string) (string, error) {
	var heads []header
	buf := `{"auth":{"passwordCredentials":{"username": "admin", "password": "` + password + `"},"tenantName": "admin"}}`
	heads = append(heads, header{"Content-Type", "application/json"})
	resp, err := getJSONResponse("POST", "http://"+url+":5000/v2.0/tokens", heads, buf)
	if err != nil {
		return "", err
	}
	lastBody = resp
	token, err := getElementValue(resp, []string{"access", "token", "id"})
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

func getElementValue(body []byte, find []string) (string, error) {
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
