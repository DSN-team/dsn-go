package main

import (
	"bufio"
	"fmt"
	"github.com/ClarkGuan/jni"
	"io"
	"log"
	"net"
	"os"
	"reflect"
	"strings"
	"unsafe"
)

// #include <stdlib.h>
// #include <stddef.h>
// #include <stdint.h>
import "C"

type databaseStr struct {
	io []byte
}

var dataStr = &databaseStr{}
var dataStrInput = &databaseStr{}
var connClient net.Conn
var workingEnv jni.Env
var workingClazz jni.Jclass
var address string

func fakeClient() {
	/*for {
		time.Sleep(1 * time.Second)
		Java_com_dsnteam_dsn_CoreManager_writeBytes([]byte("testing\n"))
	}*/
	connClient, _ = net.Dial("tcp", address)

	clientReader := bufio.NewReader(os.Stdin)

	serverReader := bufio.NewReader(connClient)
	for {
		// Waiting for the client request
		clientRequest, err := clientReader.ReadString('\n')

		switch err {
		case nil:
			clientRequest := strings.TrimSpace(clientRequest)
			if _, err = connClient.Write([]byte(clientRequest + "\n")); err != nil {
				log.Printf("failed to send the client request: %v\n", err)
			}
		case io.EOF:
			log.Println("client closed the connection")
			return
		default:
			log.Printf("client error: %v\n", err)
			return
		}

		// Waiting for the server response
		serverResponse, err := serverReader.ReadString('\n')

		switch err {
		case nil:
			log.Println(strings.TrimSpace(serverResponse))
		case io.EOF:
			log.Println("server closed the connection")
			return
		default:
			log.Printf("server error: %v\n", err)
			return
		}
	}
}
func main() {
	address = ":8080"
	go Java_com_dsnteam_dsn_CoreManager_runClient(0, 0)
	fakeClient()
}

//export Java_com_dsnteam_dsn_CoreManager_connectionTarget
func Java_com_dsnteam_dsn_CoreManager_connectionTarget(env uintptr, clazz uintptr, stringIn uintptr) {
	address = string(jni.Env(env).GetStringUTF(stringIn))
}

//export Java_com_dsnteam_dsn_CoreManager_runClient
func Java_com_dsnteam_dsn_CoreManager_runClient(env uintptr, clazz uintptr) {
	if env != 0 {
		workingEnv = jni.Env(env)
	}
	if clazz != 0 {
		workingClazz = clazz
	}
	go fakeClient()
}

//Инициализировать структуры и подключение
//export Java_com_dsnteam_dsn_CoreManager_runAnalyzer
func Java_com_dsnteam_dsn_CoreManager_runAnalyzer(env uintptr, clazz uintptr) {
	if env != 0 {
		workingEnv = jni.Env(env)
	}
	if clazz != 0 {
		workingClazz = clazz
	}
	/*for {

		println("")
	}*/
	ln, err := net.Listen("tcp", address)
	defer ln.Close()
	if err != nil {
		log.Fatalln(err)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatalln(err)
		}
		go handleConnection(conn)
	}
}

func handleConnection(con net.Conn) {
	defer con.Close()
	println("handling")
	clientReader := bufio.NewReader(con)
	println("bufio")
	for {
		// Waiting for the client request
		println("reading")
		clientRequest, err := clientReader.ReadString('\n')
		switch err {
		case nil:
			clientRequest := strings.TrimSpace(clientRequest)
			if clientRequest == ":QUIT" {
				log.Println("client requested server to close the connection so closing")
				return
			} else {
				log.Println(clientRequest)
			}
		case io.EOF:
			log.Println("client closed the connection by terminating the process")
			return
		default:
			log.Printf("error: %v\n", err)
			return
		}

		// Responding to the client request
		dataStr.io = []byte(clientRequest)
		updateCall()
		if _, err = con.Write([]byte("GOT IT!\n")); err != nil {
			log.Printf("failed to respond to client: %v\n", err)
		}
	}

}

//export Java_com_dsnteam_dsn_CoreManager_writeBytes
func Java_com_dsnteam_dsn_CoreManager_writeBytes(env uintptr, clazz uintptr, inBuffer uintptr) {
	point := jni.Env(env).GetDirectBufferAddress(inBuffer)
	size := jni.Env(env).GetDirectBufferCapacity(inBuffer)
	var data []byte

	sh := (*reflect.SliceHeader)(unsafe.Pointer(&data))
	sh.Data = uintptr(point)
	sh.Len = size
	sh.Cap = size
	dataStrInput.io = data
	_, _ = fmt.Fprint(connClient, dataStrInput.io)
}

//export Java_com_dsnteam_dsn_CoreManager_exportBytes
func Java_com_dsnteam_dsn_CoreManager_exportBytes(env uintptr, clazz uintptr) uintptr {
	buffer := jni.Env(env).NewDirectByteBuffer(unsafe.Pointer(&dataStr.io), len(dataStr.io))
	return buffer
}

//Realisation for platform
func updateCall() {
	//Call Application to read structure and update internal data interpretations, update UI.

	//Test
	println((dataStr.io))

	methodid := workingEnv.GetStaticMethodID(workingClazz, "getUpdateCallBack", "(L)")
	workingEnv.CallStaticObjectMethodA(workingClazz, methodid)
}
