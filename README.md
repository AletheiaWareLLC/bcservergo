bcservergo
==========

This is a Go implementation of a BC server using the BC data structures.

Build
=====

    go build

Setup
=====

This guide will demonstrate how to setup BC on a remote server, such as a Digital Ocean Droplet running Ubuntu 18.04 x64.

Create a new droplet and ssh into the IP address

    ssh root@your_server_ip

Firewall (UFW)

    # Install firewall
    apt install ufw
    # Allow http port
    ufw allow 80
    # Allow https port
    ufw allow 443
    # Allow bc ports
    ufw allow 22222
    ufw allow 22322
    ufw allow 23232
    # Enable firewall
    ufw enable

HTTPS (Let's Encrypt)

    # Install certbot
    apt install certbot
    certbot certonly --standalone -d your_server_domain

BC

    # Create bc user
    adduser your_server_alias
    # Create bc directory
    mkdir -p /home/your_server_alias/bc/

    # From your development machine
    # Copy server binary
    rsync $GOPATH/bin/bcservergo-linux-amd64 your_server_alias@your_server_ip:~/bc/
    # Copy website content
    rsync -r $GOPATH/src/github.com/AletheiaWareLLC/bcservergo/html your_server_alias@your_server_ip:~/bc/
    # Copy client binaries into website static content
    rsync $GOPATH/bin/bcclientgo-* your_server_alias@your_server_ip:~/bc/html/static/

    # Initialize BC
    ALIAS=your_server_alias CACHE=~/bc/cache/ KEYSTORE=~/bc/keys/ LOGSTORE=~/bc/logs/ ~/bc/html/static/bcclient-linux-amd64 init

    # Allow bcservergo to read security credentials
    chown -R your_server_alias:your_server_alias /etc/letsencrypt/
    # Allow bcservergo to bind to port 443 (HTTPS)
    # This is required each time the server binary is updated
    setcap CAP_NET_BIND_SERVICE=+eip /home/your_server_alias/bc/bcservergo-linux-amd64

Service (Systemd)

    # Create bc config
    cat > /home/your_server_alias/bc/config <<EOF
    >ALIAS=your_server_alias
    >PASSWORD='VWXYZ'
    >CACHE=cache/
    >KEYSTORE=keys/
    >LOGSTORE=logs/
    >SECURITYSTORE=/etc/letsencrypt/live/your_server_domain/
    >PEERS=bc.aletheiaware.com
    >EOF

    # Create bc service
    cat > /etc/systemd/system/bc.service <<EOF
    >[Unit]
    >Description=BC Server
    >[Service]
    >User=your_server_alias
    >WorkingDirectory=/home/your_server_alias/bc
    >EnvironmentFile=/home/your_server_alias/bc/config
    >ExecStart=/home/your_server_alias/bc/bcservergo-linux-amd64
    >SuccessExitStatus=143
    >TimeoutStopSec=10
    >Restart=on-failure
    >RestartSec=5
    >[Install]
    >WantedBy=multi-user.target
    >EOF
    # Reload daemon
    systemctl daemon-reload
    # Start service
    systemctl start bc
    # Monitor service
    journalctl -u bc
