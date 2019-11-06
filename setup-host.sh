#!/bin/bash
#
# Copyright 2019 Aletheia Ware LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e
set -x

read -p 'Username: ' USERNAME
read -p -s 'Password: ' PASSWORD
read -p 'Domain Name: ' DOMAIN

# FIXME if user already exists
# Create user
adduser ${USERNAME}

# Add user to sudoers
usermod -aG sudo ${USERNAME}

# Become the new user
su - ${USERNAME}

# Update
sudo apt update

# Upgrade
sudo apt upgrade -y

# Install dependancies
sudo apt install certbot rsync ufw

# Allow http port
sudo ufw allow 80

# Allow https port
sudo ufw allow 443

# Allow bc ports
sudo ufw allow 22222
sudo ufw allow 22322
sudo ufw allow 23232

# Enable firewall
sudo ufw enable

# Generate certificate
sudo certbot certonly --standalone -d ${DOMAIN}

# Allow bcservergo to read security credentials
sudo chown -R ${USERNAME}:${USERNAME} /etc/letsencrypt/

# Add cron job to renew certificate on the first day of the week
(sudo crontab -l ; echo '* * * * 0 sudo certbot renew --pre-hook "systemctl stop bc" --post-hook "systemctl start bc"') | sudo crontab -

# Create bc directory
mkdir -p /home/${USERNAME}/bc/

# Move into directory
cd /home/${USERNAME}/bc/

# Download server binary
curl -OL https://github.com/AletheiaWareLLC/bcservergo/releases/latest/download/bcservergo-linux-amd64

# Download website content
curl -OL https://github.com/AletheiaWareLLC/bcservergo/releases/latest/download/html.zip

# Extract zip
unzip html.zip

# Delete zip
rm html.zip

# Initialize BC
ALIAS=${DOMAIN} PASSWORD=${PASSWORD} ROOT_DIRECTORY=~/bc/ ./bcservergo-linux-amd64 init

# Allow bcservergo to bind to port 443 (HTTPS)
# This is required each time the server binary is updated
sudo setcap CAP_NET_BIND_SERVICE=+eip /home/${USERNAME}/bc/bcservergo-linux-amd64

# Create bc config
cat <<EOT >> /home/${USERNAME}/bc/config
ALIAS=${DOMAIN}
PASSWORD='${PASSWORD}'
ROOT_DIRECTORY=/home/${USERNAME}/bc/
CERTIFICATE_DIRECTORY=/etc/letsencrypt/live/${DOMAIN}/
EOT
chmod 600 /home/${USERNAME}/bc/config

# Create bc service
sudo cat <<EOT >> /etc/systemd/system/bc.service
[Unit]
Description=BC Server
[Service]
User=${USERNAME}
WorkingDirectory=/home/${USERNAME}/bc
EnvironmentFile=/home/${USERNAME}/bc/config
ExecStart=/home/${USERNAME}/bc/bcservergo-linux-amd64 start
SuccessExitStatus=143
TimeoutStopSec=10
Restart=on-failure
RestartSec=5
[Install]
WantedBy=multi-user.target
EOT

# Reload daemon
sudo systemctl daemon-reload

# Enable service
sudo systemctl enable bc

# Start service
sudo systemctl start bc
