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
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"github.com/AletheiaWareLLC/aliasgo"
	"github.com/AletheiaWareLLC/aliasservergo"
	"github.com/AletheiaWareLLC/bcgo"
	"github.com/AletheiaWareLLC/bcnetgo"
	"github.com/AletheiaWareLLC/cryptogo"
	"github.com/AletheiaWareLLC/netgo"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"time"
)

type Server struct {
	Root     string
	Cert     string
	Cache    *bcgo.FileCache
	Network  bcgo.Network
	Listener bcgo.MiningListener
}

func (s *Server) Init() (*bcgo.Node, error) {
	// Add BC host to peers
	if err := bcgo.AddPeer(s.Root, bcgo.GetBCHost()); err != nil {
		return nil, err
	}

	// Create Node
	node, err := bcgo.GetNode(s.Root, s.Cache, s.Network)
	if err != nil {
		return nil, err
	}

	// Register Alias
	if err := aliasgo.Register(node, s.Listener); err != nil {
		return nil, err
	}

	return node, nil
}

func (s *Server) Start(node *bcgo.Node) error {
	// Open channels
	aliases := aliasgo.OpenAliasChannel()

	for _, c := range []*bcgo.Channel{
		aliases,
	} {
		// Load channel
		if err := c.LoadCachedHead(s.Cache); err != nil {
			log.Println(err)
		}
		// Pull channel
		if err := c.Pull(s.Cache, s.Network); err != nil {
			log.Println(err)
		}
		// Add channel to node
		node.AddChannel(c)
	}

	// Serve Block Requests
	go bcnetgo.Bind(bcgo.PORT_GET_BLOCK, bcnetgo.BlockPortHandler(s.Cache, s.Network))
	// Serve Head Requests
	go bcnetgo.Bind(bcgo.PORT_GET_HEAD, bcnetgo.HeadPortHandler(s.Cache, s.Network))
	// Serve Block Updates
	go bcnetgo.Bind(bcgo.PORT_BROADCAST, bcnetgo.BroadcastPortHandler(s.Cache, s.Network, node.GetChannel))

	// Redirect HTTP Requests to HTTPS
	go http.ListenAndServe(":80", http.HandlerFunc(netgo.HTTPSRedirect(map[string]bool{
		"/":               true,
		"/alias":          true,
		"/alias-register": true,
		"/block":          true,
		"/channel":        true,
		"/channels":       true,
		"/keys":           true,
	})))

	// Serve Web Requests
	mux := http.NewServeMux()
	mux.HandleFunc("/", netgo.StaticHandler("html/static"))
	aliasTemplate, err := template.ParseFiles("html/template/alias.html")
	if err != nil {
		return err
	}
	mux.HandleFunc("/alias", aliasservergo.AliasHandler(aliases, s.Cache, s.Network, aliasTemplate))
	aliasRegistrationTemplate, err := template.ParseFiles("html/template/alias-register.html")
	if err != nil {
		return err
	}
	mux.HandleFunc("/alias-register", aliasservergo.AliasRegistrationHandler(aliases, node, aliasgo.ALIAS_THRESHOLD, s.Listener, aliasRegistrationTemplate))
	blockTemplate, err := template.ParseFiles("html/template/block.html")
	if err != nil {
		return err
	}
	mux.HandleFunc("/block", bcnetgo.BlockHandler(s.Cache, s.Network, blockTemplate))
	channelTemplate, err := template.ParseFiles("html/template/channel.html")
	if err != nil {
		return err
	}
	mux.HandleFunc("/channel", bcnetgo.ChannelHandler(s.Cache, s.Network, channelTemplate))
	channelListTemplate, err := template.ParseFiles("html/template/channel-list.html")
	if err != nil {
		return err
	}
	mux.HandleFunc("/channels", bcnetgo.ChannelListHandler(s.Cache, s.Network, channelListTemplate, node.GetChannels))
	mux.HandleFunc("/keys", bcnetgo.KeyShareHandler(make(bcnetgo.KeyShareStore), 2*time.Minute))
	// Serve HTTPS Requests
	config := &tls.Config{MinVersion: tls.VersionTLS10}
	server := &http.Server{Addr: ":443", Handler: mux, TLSConfig: config}
	return server.ListenAndServeTLS(path.Join(s.Cert, "fullchain.pem"), path.Join(s.Cert, "privkey.pem"))
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
			log.Println(node.Alias)
			publicKeyBytes, err := cryptogo.RSAPublicKeyToPKIXBytes(&node.Key.PublicKey)
			if err != nil {
				log.Println(err)
				return
			}
			log.Println(base64.RawURLEncoding.EncodeToString(publicKeyBytes))
		case "start":
			node, err := bcgo.GetNode(s.Root, s.Cache, s.Network)
			if err != nil {
				log.Println(err)
				return
			}
			if err := s.Start(node); err != nil {
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
	fmt.Fprintln(output, "\tbcserver - display usage")
	fmt.Fprintln(output, "\tbcserver init - initializes environment, generates key pair, and registers alias")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "\tbcserver start - starts the server")
}

func PrintLegalese(output io.Writer) {
	fmt.Fprintln(output, "BC Legalese:")
	fmt.Fprintln(output, "BC is made available by Aletheia Ware LLC [https://aletheiaware.com] under the Terms of Service [https://aletheiaware.com/terms-of-service.html] and Privacy Policy [https://aletheiaware.com/privacy-policy.html].")
	fmt.Fprintln(output, "By continuing to use this software you agree to the Terms of Service, and Privacy Policy.")
}

func main() {
	rootDir, err := bcgo.GetRootDirectory()
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("Root Directory:", rootDir)

	logFile, err := bcgo.SetupLogging(rootDir)
	if err != nil {
		log.Println(err)
		return
	}
	defer logFile.Close()
	log.Println("Log File:", logFile.Name())

	certDir, err := bcgo.GetCertificateDirectory(rootDir)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("Certificate Directory:", certDir)

	cacheDir, err := bcgo.GetCacheDirectory(rootDir)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("Cache Directory:", cacheDir)

	cache, err := bcgo.NewFileCache(cacheDir)
	if err != nil {
		log.Println(err)
		return
	}

	peers, err := bcgo.GetPeers(rootDir)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("Peers:", peers)

	network := &bcgo.TcpNetwork{
		Peers: peers,
	}

	server := &Server{
		Root:     rootDir,
		Cert:     certDir,
		Cache:    cache,
		Network:  network,
		Listener: &bcgo.PrintingMiningListener{Output: os.Stdout},
	}

	server.Handle(os.Args[1:])
}
