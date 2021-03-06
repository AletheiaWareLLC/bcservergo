/*
 * Copyright 2019 Aletheia Ware LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"aletheiaware.com/aliasgo"
	"aletheiaware.com/aliasservergo"
	"aletheiaware.com/bcgo"
	"aletheiaware.com/bcgo/account"
	"aletheiaware.com/bcgo/cache"
	"aletheiaware.com/bcgo/channel"
	"aletheiaware.com/bcgo/network"
	"aletheiaware.com/bcgo/node"
	"aletheiaware.com/bcnetgo"
	"aletheiaware.com/cryptogo"
	"aletheiaware.com/netgo"
	"crypto/tls"
	"encoding/base64"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

var (
	c = flag.String("channel", "", "BC channel")
	p = flag.String("peer", "", "BC peer")
)

type Server struct {
	Root     string
	Cert     string
	Cache    *cache.FileSystem
	Network  *network.TCP
	Listener bcgo.MiningListener
}

func (s *Server) Init() (bcgo.Node, error) {
	// Load Account
	account, err := account.LoadRSA(s.Root)
	if err != nil {
		return nil, err
	}

	// Create Node
	node := node.New(account, s.Cache, s.Network)

	// Register Alias
	if err := aliasgo.Register(node, s.Listener); err != nil {
		return nil, err
	}

	return node, nil
}

func (s *Server) Start(node bcgo.Node) error {
	// Open channels
	aliases := aliasgo.OpenAliasChannel()

	channels := []bcgo.Channel{
		aliases,
	}
	if *c != "" {
		for _, c := range bcgo.SplitRemoveEmpty(*c, ",") {
			parts := strings.Split(c, ":")
			if len(parts) > 0 {
				name := parts[0]
				if name != aliasgo.ALIAS {
					if len(parts) > 1 {
						threshold, err := strconv.Atoi(parts[1])
						if err != nil {
							return err
						}
						channels = append(channels, channel.NewPoW(name, uint64(threshold)))
					} else {
						channels = append(channels, channel.New(name))
					}
				}
			}
		}
	}

	for _, c := range channels {
		go func() {
			if err := c.Refresh(s.Cache, s.Network); err != nil {
				log.Println(err)
			}
		}()
		// Add channel to node
		node.AddChannel(c)
	}

	// Serve BC Requests
	bcnetgo.BindAllTCP(s.Cache, s.Network, node.Channel)

	// Serve Web Requests
	mux := http.NewServeMux()
	mux.HandleFunc("/", netgo.StaticHandler("html/static"))
	aliasTemplate, err := template.ParseFiles("html/template/alias.go.html")
	if err != nil {
		return err
	}
	mux.HandleFunc("/alias", aliasservergo.AliasHandler(aliases, s.Cache, aliasTemplate))
	aliasRegistrationTemplate, err := template.ParseFiles("html/template/alias-register.go.html")
	if err != nil {
		return err
	}
	mux.HandleFunc("/alias-register", aliasservergo.AliasRegistrationHandler(aliases, node, aliasgo.ALIAS_THRESHOLD, s.Listener, aliasRegistrationTemplate))
	blockTemplate, err := template.ParseFiles("html/template/block.go.html")
	if err != nil {
		return err
	}
	mux.HandleFunc("/block", bcnetgo.BlockHandler(s.Cache, blockTemplate))
	channelTemplate, err := template.ParseFiles("html/template/channel.go.html")
	if err != nil {
		return err
	}
	mux.HandleFunc("/channel", bcnetgo.ChannelHandler(s.Cache, channelTemplate))
	channelListTemplate, err := template.ParseFiles("html/template/channel-list.go.html")
	if err != nil {
		return err
	}
	mux.HandleFunc("/channels", bcnetgo.ChannelListHandler(s.Cache, channelListTemplate, node.Channels))
	mux.HandleFunc("/keys", cryptogo.KeyShareHandler(make(cryptogo.KeyShareStore), 2*time.Minute))

	if bcgo.BooleanFlag(netgo.HTTPS) {
		// Redirect HTTP Requests to HTTPS
		go http.ListenAndServe(":80", http.HandlerFunc(netgo.HTTPSRedirect(node.Account().Alias(), map[string]bool{
			"/":               true,
			"/alias":          true,
			"/alias-register": true,
			"/block":          true,
			"/channel":        true,
			"/channels":       true,
			"/keys":           true,
		})))

		// Serve HTTPS Requests
		config := &tls.Config{MinVersion: tls.VersionTLS10}
		server := &http.Server{Addr: ":443", Handler: mux, TLSConfig: config}
		return server.ListenAndServeTLS(path.Join(s.Cert, "fullchain.pem"), path.Join(s.Cert, "privkey.pem"))
	} else {
		log.Println("HTTP Server Listening on :80")
		return http.ListenAndServe(":80", mux)
	}
}

func (s *Server) Handle(args []string) {
	if len(args) > 0 {
		switch args[0] {
		case "init":
			PrintLegalese(os.Stdout)
			node, err := s.Init()
			if err != nil {
				log.Println(err)
				return
			}
			log.Println("Initialized")
			log.Println(node.Account().Alias())
			format, bytes, err := node.Account().PublicKey()
			if err != nil {
				log.Println(err)
				return
			}
			log.Println(base64.RawURLEncoding.EncodeToString(bytes))
			log.Println(format)
		case "start":
			account, err := account.LoadRSA(s.Root)
			if err != nil {
				log.Println(err)
				return
			}
			if err := s.Start(node.New(account, s.Cache, s.Network)); err != nil {
				log.Println(err)
				return
			}
		default:
			log.Println("Cannot handle", args[0])
		}
	} else {
		PrintUsage(os.Stdout)
	}
}

func PrintUsage(output io.Writer) {
	fmt.Fprintln(output, "BC Server Usage:")
	fmt.Fprintf(output, "\t%s - display usage\n", os.Args[0])
	fmt.Fprintf(output, "\t%s init - initializes environment, generates key pair, and registers alias\n", os.Args[0])
	fmt.Fprintln(output)
	fmt.Fprintf(output, "\t%s start - starts the server\n", os.Args[0])
}

func PrintLegalese(output io.Writer) {
	fmt.Fprintln(output, "BC Legalese:")
	fmt.Fprintln(output, "BC is made available by Aletheia Ware LLC [https://aletheiaware.com] under the Terms of Service [https://aletheiaware.com/terms-of-service.html] and Privacy Policy [https://aletheiaware.com/privacy-policy.html].")
	fmt.Fprintln(output, "By continuing to use this software you agree to the Terms of Service, and Privacy Policy.")
}

func main() {
	// Parse command line flags
	flag.Parse()

	// Load config files (if any)
	err := bcgo.LoadConfig()
	if err != nil {
		log.Fatal("Could not load config:", err)
	}

	// Get root directory
	rootDir, err := bcgo.RootDirectory()
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("Root Directory:", rootDir)

	// Setup logging
	logFile, err := bcgo.SetupLogging(rootDir)
	if err != nil {
		log.Println(err)
		return
	}
	defer logFile.Close()
	log.Println("Log File:", logFile.Name())

	// Get certificate directory
	certDir, err := bcgo.CertificateDirectory(rootDir)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("Certificate Directory:", certDir)

	// Get cache directory
	cacheDir, err := bcgo.CacheDirectory(rootDir)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("Cache Directory:", cacheDir)

	// Create file cache
	cache, err := cache.NewFileSystem(cacheDir)
	if err != nil {
		log.Println(err)
		return
	}

	var peers []string
	if *p == "" {
		// Get peers
		peers, err = bcgo.Peers(rootDir)
		if err != nil {
			log.Fatal("Could not get network peers:", err)
		}
		if len(peers) == 0 {
			host := bcgo.BCHost()
			alias, err := bcgo.Alias()
			if err != nil {
				log.Println(err)
			}
			if host != alias {
				peers = append(peers, host)
			}
		}
	} else {
		peers = bcgo.SplitRemoveEmpty(*p, ",")
	}
	log.Println("Peers:", peers)

	// Create network of peers
	network := network.NewTCP(peers...)

	server := &Server{
		Root:     rootDir,
		Cert:     certDir,
		Cache:    cache,
		Network:  network,
		Listener: &bcgo.LoggingMiningListener{},
	}

	server.Handle(flag.Args())
}
