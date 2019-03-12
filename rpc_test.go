package main

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"
)
func rpcRequest(){

}
func TestHello(t *testing.T){
	jsonByte := []byte(`{"id":1,"method":"version","params":[],"jsonrpc":"2.0"}`)
	req,_ := http.NewRequest("post","http://localhost:8009",bytes.NewBuffer(jsonByte))
	req.Header.Set("Content-Type","application/json")
	client := http.Client{}
	resp,_ := client.Do(req)
	body,_ := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	t.Log(string(body))

}
