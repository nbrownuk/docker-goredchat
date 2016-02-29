package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/soveran/redisurl"
)

func main() {

	var username string

	// Define help message
	flag.Usage = func() {
		fmt.Printf("Usage: %s [-r URL] username\n", os.Args[0])
		fmt.Printf("  e.g. %s -r redis://redis_svr:6379 antirez\n", os.Args[0])
		fmt.Printf("\n  If -r URL is not used, the REDIS_URL env must be set instead\n")
	}

	// Parse command line arguments and check for REDIS_URL in absence of -r flag
	url := flag.String("r", "", "URL of Redis server")
	flag.Parse()

	if len(*url) == 0 {
		*url = os.Getenv("REDIS_URL")
		if len(*url) == 0 {
			fmt.Println("a URL must be specified")
			flag.Usage()
			os.Exit(1)
		}
	}

	// Make sure there is a username
	args := flag.Args()
	if len(args) == 1 {
		username = args[0]
	} else {
		fmt.Println("a single, unique username must be provided")
		flag.Usage()
		os.Exit(1)
	}

	// Now we connect to the Redis server
	conn, err := redisurl.ConnectToURL(*url)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer conn.Close()

	// We make a key and set it with the username as a value and a time to live
	// this will be the lock on the username and if we can't set it, its a name clash

	userkey := "online." + username
	val, err := conn.Do("SET", userkey, username, "NX", "EX", "120")

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if val == nil {
		fmt.Println("User already online")
		os.Exit(1)
	}

	val, err = conn.Do("SADD", "users", username)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if val == nil {
		fmt.Println("User still in online set")
		os.Exit(1)
	}

	// A ticker will let us update our presence on the Redis server
	tickerChan := time.NewTicker(time.Second * 60).C

	// Now we create a channel and go routine that'll subscribe to our published messages
	// We'll give it its own connection because subscribes like to have their own connection
	subChan := make(chan string)
	go func() {
		subconn, err := redisurl.ConnectToURL(*url)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		defer subconn.Close()

		psc := redis.PubSubConn{Conn: subconn}
		psc.Subscribe("messages")
		for {
			switch v := psc.Receive().(type) {
			case redis.Message:
				subChan <- string(v.Data)
			case redis.Subscription:
				// We don't need to listen to subscription messages,
			case error:
				return
			}
		}
	}()

	// Display a welcome/use message
	fmt.Printf("\nWelcome to goredchat %s! Type /who to see who's online, /exit to exit.\n\n", username)

	// Now we'll make a simple channel and go routine that listens for complete lines from the user
	// When a complete line is entered, it'll be delivered to the channel.
	sayChan := make(chan string)
	go func() {
		bio := bufio.NewReader(os.Stdin)
		for {
			line, _, err := bio.ReadLine()
			if err != nil {
				fmt.Println(err)
				sayChan <- "/exit"
				return
			}
			sayChan <- string(line)
		}
	}()

	conn.Do("PUBLISH", "messages", username+" has joined")

	chatExit := false

	msgHead := username + ": "

	for !chatExit {
		select {
		case msg := <-subChan:
			if !strings.Contains(msg, msgHead) {
				fmt.Println(msg)
			}
		case <-tickerChan:
			val, err = conn.Do("SET", userkey, username, "XX", "EX", "120")
			if err != nil || val == nil {
				fmt.Println("Heartbeat set failed")
				chatExit = true
			}
		case line := <-sayChan:
			if line == "/exit" {
				chatExit = true
			} else if line == "/who" {
				names, _ := redis.Strings(conn.Do("SMEMBERS", "users"))
				for _, name := range names {
					fmt.Println(name)
				}
			} else {
				conn.Do("PUBLISH", "messages", username+": "+line)
			}
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}

	// We're leaving so let's delete the userkey and remove the username from the online set
	conn.Do("DEL", userkey)
	conn.Do("SREM", "users", username)
	conn.Do("PUBLISH", "messages", username+" has left")

}
