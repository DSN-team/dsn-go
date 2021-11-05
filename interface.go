package core

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"runtime"
	"time"
)

var UpdateUI func(int, int)

type strBuffer struct {
	Io []byte
}

var DataStrOutput = &strBuffer{}
var DataStrInput = &strBuffer{}
var Profiles []ShowProfile

func testAES() {

}

func (cur Profile) Register(username, password string) bool {
	key := genProfileKey()
	if key == nil {
		return false
	}
	cur = Profile{Username: username, Password: password, PrivateKey: key}
	log.Println(cur)
	addProfile(cur)
	return true
}

func (cur Profile) Login(password string, pos int) (result bool) {
	var privateKeyEncBytes []byte
	cur.Id = Profiles[pos].Id
	cur.Username, cur.Address, privateKeyEncBytes = getProfileByID(Profiles[pos].Id)
	if privateKeyEncBytes == nil {
		return false
	}
	result = cur.decProfileKey(privateKeyEncBytes, password)
	fmt.Println("Login status:", result)
	return
}
func UsernamePos(username string) int {
	profiles := getProfiles()
	pos := -1
	for i := 0; i < len(profiles); i++ {
		if profiles[i].Username == username {
			pos = i
			break
		}
	}
	return pos
}
func LoadProfiles() int {
	Profiles = getProfiles()
	return len(Profiles)
}

func (cur Profile) GetProfilePublicKey() string {
	return EncPublicKey(MarshalPublicKey(&cur.PrivateKey.PublicKey))
}

func (cur Profile) AddFriend(username, address, publicKey string) {
	decryptedPublicKey := UnmarshalPublicKey(DecPublicKey(publicKey))
	id := cur.searchUser(username)
	user := User{Username: username, Address: address, PublicKey: &decryptedPublicKey, IsFriend: true}
	if id == -1 {
		cur.addUser(user)
	} else {
		cur.editUser(id, user)
	}
}

func (cur Profile) LoadFriends() int {
	println("Loading Friends from db")
	cur.Friends = cur.getFriends()
	return len(cur.Friends)
}

func (cur Profile) ConnectToFriends() {
	for i := 0; i < len(cur.Friends); i++ {
		go cur.connect(i)
	}
}

func (cur Profile) ConnectToFriend(userId int) {
	for i := 0; i < len(cur.Friends); i++ {
		go func(index int) {
			if cur.Friends[index].Id == userId {
				cur.connect(index)
				return
			}
		}(i)
	}
}

func (cur Profile) RunServer(address string) {
	go cur.server(address)
}

func (cur Profile) WriteBytes(userId, lenIn int) {
	var con net.Conn
	if _, ok := cur.connections.Load(userId); !ok {
		log.Println("Not connected to:", userId)
		return
	}
	value, _ := cur.connections.Load(userId)
	con = value.(net.Conn)
	runtime.KeepAlive(DataStrInput.Io)
	log.Println("writing to:", con.RemoteAddr())

	log.Println("input:", DataStrInput.Io)
	println("input str:", string(DataStrInput.Io))

	switch err {
	case nil:
		bs := make([]byte, 9)
		binary.BigEndian.PutUint64(bs, uint64(lenIn))
		bs[8] = '\n'
		bytes := append(bs, DataStrInput.Io...)
		println("ClientSend:", bytes, " count:", lenIn)

		if _, err = con.Write(bytes); err != nil {
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

func (cur Profile) connect(pos int) {
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

	if _, ok := cur.connections.Load(targetId); !ok {
		log.Println("connection not found adding...")
		cur.connections.Store(targetId, con)
	} else {
		log.Println("connection already connected")
		return
	}

	println("connected to target", targetId)
	go handleConnection(targetId, con)
}

func (cur Profile) server(address string) {
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

		if _, ok := cur.connections.Load(clientId); !ok {
			log.Println("connection not found adding...")
			cur.connections.Store(clientId, con)
		} else {
			log.Println("connection already connected")
			return
		}

		go handleConnection(clientId, con)
	}
}

//Symmetrical connection for TCP between f2f
func handleConnection(clientId int, con net.Conn) {
	log.Println("handling")

	defer func(con net.Conn) {
		err := con.Close()
		ErrHandler(err)
	}(con)

	clientReader := bufio.NewReader(con)

	for {
		// Waiting for the client request
		log.Println("reading")
		state, err := clientReader.Peek(9)
		ErrHandler(err)
		_, err = clientReader.Discard(9)
		ErrHandler(err)
		count := binary.BigEndian.Uint64(state[0:8])
		log.Println("Count:", count)
		DataStrOutput.Io, err = clientReader.Peek(int(count))
		ErrHandler(err)
		_, err = clientReader.Discard(int(count))
		switch err {
		case nil:
			log.Println(DataStrOutput.Io)
		case io.EOF:
			log.Println("client closed the connection by terminating the process")
			return
		default:
			log.Printf("error: %v\n", err)
			return
		}

		log.Println("updating callback")
		//updateCall(int(count), clientId)
		UpdateUI(int(count), clientId)
	}
}
