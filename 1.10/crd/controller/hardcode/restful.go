package main

import (
	"fmt"
	"net/http"
	"crypto/tls"
	"io/ioutil"
	"encoding/json"
	"strings"
	"bytes"
	"time"
)

const yaml = 
`apiVersion: v1
kind: Pod
metadata:
  name: tensorflow-REPLACE_POD_NAME
spec:
  containers:
  - name: container1
    image: REPLACE_IMAGE_NAME
`

const patchjson = 
`{
	"spec": {
		"containers": [
			{ "name": "container1", "image": "REPLACE_IMAGE_NAME" }
		]
	}
}
`

var client *http.Client
var finished chan int = make(chan int)
var TOKEN string

func main() {
	readToken()
	client = &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true},}}
	for {
		go controller()
		time.Sleep(time.Second)
		<- finished
	}
}

func readToken() {
	f, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
    if err != nil {
        fmt.Print(err)
    }
    TOKEN = string(f)
}

func controller() {
	calculate_differnet(get_tf_list(), get_pod_list())
	finished <- 1
}

func calculate_differnet(desired_state []map[string]string, current_state []map[string]string) {
	var create []map[string]string
	var update []map[string]string
	var delete []map[string]string
	for _, current := range current_state {
		isFind := false
		for _, desired := range desired_state {
			if current["job"] == desired["job"] {
				isFind = true
				break
			}
		}
		if !isFind {
			delete = append(delete, current)
		}
	}

	for _, desired := range desired_state {
		isFind := false
		for _, current := range current_state {
			if desired["job"] == current["job"] {
				if desired["image"] != current["image"] {
					update = append(update, desired)
				}
				isFind = true
				break
			}
		}
		if !isFind {
			create = append(create, desired)
		}
	}

	url := "https://kubernetes/api/v1/namespaces/default/pods"

	for _, object := range create {
		body := strings.Replace(strings.Replace(yaml, "REPLACE_POD_NAME", object["job"], 1), "REPLACE_IMAGE_NAME", object["image"], 1)
		reqest, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(body)))
		if err != nil {
			panic(err)
		}
		reqest.Header.Set("Content-Type", "application/yaml;charset=utf-8")
		reqest.Header.Set("Authorization", "Bearer " + TOKEN)
		resp, err := client.Do(reqest)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()
		/*result_body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		fmt.Println(string(result_body))*/
		fmt.Println("create")
	}

	for _, object := range delete {
		reqest, err := http.NewRequest("DELETE", url+"/tensorflow-"+object["job"], nil)
		if err != nil {
			panic(err)
		}
		reqest.Header.Set("Authorization", "Bearer " + TOKEN)
		resp, err := client.Do(reqest)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()
		/*result_body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		fmt.Println(string(result_body))*/
		fmt.Println("delete")
	}

	for _, object := range update {
		body := strings.Replace(patchjson, "REPLACE_IMAGE_NAME", object["image"], 1)
		reqest, err := http.NewRequest("PATCH", url+"/tensorflow-"+object["job"], bytes.NewBuffer([]byte(body)))
		if err != nil {
			panic(err)
		}
		reqest.Header.Set("Content-Type", "application/strategic-merge-patch+json;charset=utf-8")
		reqest.Header.Set("Authorization", "Bearer " + TOKEN)
		resp, err := client.Do(reqest)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()
		/*result_body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		fmt.Println(string(result_body))*/
		fmt.Println("update")
	}
}

func get_tf_list() []map[string]string {
	url := "https://kubernetes/apis/lsalab.nthu.edu.tw/v1/namespaces/default/tensorflows"
	reqest, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}
	reqest.Header.Set("Authorization", "Bearer " + TOKEN)
	resp, err := client.Do(reqest)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	result_body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	result_json := map[string]interface{}{}

	err = json.Unmarshal([]byte(result_body), &result_json)
	if err != nil {
		panic(err)
	}

	var specs []map[string]string
	for _, object := range result_json["items"].([]interface{}) {
		tf_job := object.(map[string]interface{})["spec"].(map[string]interface{})["job"].(string)
		tf_image := object.(map[string]interface{})["spec"].(map[string]interface{})["image"].(string)

		specs = append(specs, map[string]string{"job":tf_job,"image":tf_image})
	}
	return specs
}

func get_pod_list() []map[string]string {
	url := "https://kubernetes/api/v1/namespaces/default/pods"
	reqest, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}
	reqest.Header.Set("Authorization", "Bearer " + TOKEN)
	resp, err := client.Do(reqest)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	result_body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	result_json := map[string]interface{}{}

	err = json.Unmarshal([]byte(result_body), &result_json)
	if err != nil {
		panic(err)
	}

	var specs []map[string]string
	for _, object := range result_json["items"].([]interface{}) {
		pod_name := object.(map[string]interface{})["metadata"].(map[string]interface{})["name"].(string)
		pod_image := object.(map[string]interface{})["spec"].(map[string]interface{})["containers"].([]interface{})[0].(map[string]interface{})["image"].(string)

		if i := strings.Index(pod_name, "tensorflow-"); i != -1 && i == 0 {
			specs = append(specs, map[string]string{"job":strings.Replace(pod_name, "tensorflow-", "", 1),"image":pod_image})
		}
	}
	return specs
}
