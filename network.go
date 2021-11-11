package core

import (
	"bufio"
	"fmt"
	"github.com/DSN-team/core/utils"
	"io"
	"log"
	"math/big"
	"net"
	"runtime"
	"time"
)

const (
	RequestData             = byte(0)
	RequestDataVerification = byte(1)
	RequestNetwork          = byte(2)
)

type NetworkInterface interface {
	startTimer()
	sendData(callback func())
}

func (cur *Profile) server(address string) {
	ln, err := net.Listen("tcp", address)
	ErrHandler(err)
	defer func(ln net.Listener) {
		err := ln.Close()
		ErrHandler(err)
	}(ln)
	ErrHandler(err)
	for {
		con, err := ln.Accept()
		ErrHandler(err)
		println("accepted server client")

		profilePublicKey := MarshalPublicKey(&cur.PrivateKey.PublicKey)

		clientReader := bufio.NewReader(con)
		publicKeyLen := len(profilePublicKey)
		println(publicKeyLen)
		clientKey, err := clientReader.Peek(publicKeyLen)
		ErrHandler(err)
		_, err = clientReader.Discard(publicKeyLen)
		ErrHandler(err)

		log.Println("reader size:", clientReader.Size())

		var clientId int

		clientPublicKeyString := EncPublicKey(clientKey)
		profilePublicKeyString := EncPublicKey(profilePublicKey)
		log.Println("Current profile public key:", profilePublicKeyString)
		log.Println("client public key:", clientPublicKeyString)

		if profilePublicKeyString != clientPublicKeyString {
			clientId = getUserByPublicKey(clientPublicKeyString)
			if clientId == 0 {
				log.Println("not found in database")
				return
			}
		}

		log.Println("connected:", clientId, clientPublicKeyString)

		if _, ok := cur.Connections.Load(clientId); !ok {
			log.Println("connection not found adding...")
			cur.Connections.Store(clientId, con)
		} else {
			log.Println("connection already connected")
			return
		}

		go cur.handleRequest(clientId, con)
	}
}

func (cur *Profile) connect(pos int) {
	log.Println("Connecting to friend:", cur.Friends[pos].Username)
	con, err := net.Dial("tcp", cur.Friends[pos].Address)
	for err != nil {
		con, err = net.Dial("tcp", cur.Friends[pos].Address)
		ErrHandler(err)
		time.Sleep(1 * time.Second)
	}

	publicKey := MarshalPublicKey(&cur.PrivateKey.PublicKey)
	_, err = con.Write(publicKey)
	ErrHandler(err)

	targetId := cur.Friends[pos].Id
	if _, ok := cur.Connections.Load(targetId); !ok {
		log.Println("connection not found adding...")
		cur.Connections.Store(targetId, con)
	} else {
		log.Println("connection already connected")
		return
	}

	println("connected to target", targetId)
	go cur.handleRequest(targetId, con)
}

func (cur *Profile) RunServer(address string) {
	go cur.server(address)
}

func (cur *Profile) BuildDataRequest(requestType byte, size uint64, data []byte, userId int) (output []byte) {
	request := make([]byte, 0)
	utils.SetByte(&request, requestType)
	utils.SetUint64(&request, size)
	utils.SetBytes(&request, cur.encryptAES(cur.Friends[userId].PublicKey, data))
	return request
}

func (cur *Profile) WriteRequest(userId int, request []byte) {
	var con net.Conn
	if _, ok := cur.Connections.Load(userId); !ok {
		log.Println("Not connected to:", userId)
		return
	}
	value, _ := cur.Connections.Load(userId)
	con = value.(net.Conn)
	runtime.KeepAlive(cur.DataStrInput)
	log.Println("writing to:", con.RemoteAddr())

	log.Println("input:", cur.DataStrInput)
	println("input str:", string(cur.DataStrInput))

	switch err {
	case nil:
		println("ClientSend:", request, " count:", len(request))
		if _, err = con.Write(request); err != nil {
			log.Printf("failed to send the client request: %v\n", err)
		}
	case io.EOF:
		log.Println("client closed the connection")
		return
	default:
		log.Printf("client error: %v\n", err)
		return
	}
}

//Symmetrical connection for TCP between f2f
func (cur *Profile) handleRequest(clientId int, con net.Conn) {
	log.Println("handling")
	defer func(con net.Conn) {
		err := con.Close()
		ErrHandler(err)
	}(con)
	clientReader := bufio.NewReader(con)
	for {
		requestType := utils.GetByte(clientReader)
		fmt.Println("Request type:", requestType)
		switch requestType {
		case RequestData:
			{
				cur.dataHandler(clientId, clientReader)
				break
			}
		case RequestNetwork:
			{
				cur.networkHandler(clientId, clientReader)
				break
			}
		case RequestDataVerification:
			{
				cur.verificationHandler(clientId, clientReader)
				break
			}
		}
	}
}

func (cur *Profile) dataHandler(clientId int, clientReader *bufio.Reader) {
	// Waiting for the client request
	count := utils.GetUint64Reader(clientReader)
	log.Println("Count:", count)
	encData, err := utils.GetBytes(clientReader, count)
	cur.DataStrOutput = cur.decryptAES(encData)
	cur.DataStrOutput = append([]byte{RequestData}, cur.DataStrOutput...)
	switch err {
	case nil:
		log.Println(cur.DataStrOutput)
	case io.EOF:
		log.Println("client closed the connection by terminating the process")
		return
	default:
		log.Printf("error: %v\n", err)
		return
	}
	log.Println("updating callback")
	UpdateUI(int(count), clientId)
}

func (cur *Profile) verificationHandler(clientId int, clientReader *bufio.Reader) {
	cur.Friends[cur.getFriendNumber(clientId)].Ping = int(utils.GetUint16Reader(clientReader))
}

func (cur *Profile) networkHandler(clientId int, clientReader *bufio.Reader) {
	//metaData sizes
	requestDepth := utils.GetUint8Reader(clientReader)
	requestDegree := utils.GetUint8Reader(clientReader)
	backTraceSize := utils.GetUint8Reader(clientReader)
	backTrace, _ := utils.GetBytes(clientReader, uint64(backTraceSize))

	userNameSize := utils.GetUint16Reader(clientReader)
	fromUserNameSize := utils.GetUint16Reader(clientReader)

	//todo complete this
	publicKey, _ := utils.GetBytes(clientReader, uint64(128))
	metaDataSize := utils.GetUint32Reader(clientReader)
	metaDataEncrypted, _ := utils.GetBytes(clientReader, uint64(metaDataSize))
	signSize := utils.GetUint32Reader(clientReader)
	signDataEncrypted, _ := utils.GetBytes(clientReader, uint64(signSize))

	signData := cur.decryptAES(signDataEncrypted)

	r := new(big.Int).SetBytes(signData[0 : signSize/2])
	s := new(big.Int).SetBytes(signData[signSize/2 : signSize/2])

	if cur.verifyData(metaDataEncrypted, *r, *s) == true {
		metaData := cur.decryptAES(metaDataEncrypted)
		username := metaData[0:userNameSize]
		fromUsername := metaData[userNameSize:fromUserNameSize]

		fmt.Println("UserNameSize:", userNameSize, " FromUserNameSize:", fromUserNameSize, " Username:", username,
			" Depth:", requestDepth, " BackTrace:", backTrace)
		if cur.ThisUser.Username == string(username) {
			fmt.Println("Friend request done, request from:", string(fromUsername), "Accept?")
			cur.DataStrOutput = append([]byte{RequestNetwork}, fromUsername...)
			cur.DataStrOutput = append(cur.DataStrOutput, publicKey...)

			cur.DataStrOutput = append(cur.DataStrOutput, backTrace...)

			UpdateUI(int(userNameSize), clientId)
			return
		}
	}

	requestDepth--
	//Required: Friends.ping && Friends.is_online
	if requestDepth > 0 {
		encrypted := make([]byte, 0)
		cur.buildEncryptedPart(&encrypted, publicKey, signData, metaDataEncrypted)
		cur.writeFindFriendRequestSecondary(int(requestDepth), int(requestDegree), clientId, backTrace, encrypted)
	}
}
