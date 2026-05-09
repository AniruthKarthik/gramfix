#!/bin/bash
set -e

echo "Installing dependencies..."

install_debian() {
    sudo apt-get update
    sudo apt-get install -y wl-clipboard xclip xsel xdotool default-jre golang wget unzip
    # wtype and languagetool might need manual install on older debian
    sudo apt-get install -y wtype || echo "wtype not found, skipping"
    sudo apt-get install -y languagetool || install_languagetool_manual
}

install_fedora() {
    sudo dnf install -y wl-clipboard xclip xsel xdotool wtype java-latest-openjdk golang wget unzip
    sudo dnf install -y languagetool || install_languagetool_manual
}

install_arch() {
    sudo pacman -Sy --noconfirm wl-clipboard xclip xsel xdotool wtype jre-openjdk go wget unzip languagetool
}

install_languagetool_manual() {
    echo "Installing LanguageTool manually to /usr/share/languagetool..."
    if [ ! -d "/usr/share/languagetool" ]; then
        wget https://languagetool.org/download/LanguageTool-stable.zip -O /tmp/LanguageTool.zip
        sudo unzip /tmp/LanguageTool.zip -d /opt/
        sudo mv /opt/LanguageTool-*/ /usr/share/languagetool
        sudo rm /tmp/LanguageTool.zip
    else
        echo "LanguageTool already installed."
    fi
}

if [ -x "$(command -v apt-get)" ]; then
    install_debian
elif [ -x "$(command -v dnf)" ]; then
    install_fedora
elif [ -x "$(command -v pacman)" ]; then
    install_arch
else
    echo "Unsupported package manager. Please install dependencies manually: wl-clipboard, xclip, xdotool, wtype, go, java, languagetool."
fi

echo "Building gramfix..."
make build

echo "Installing gramfix to /usr/local/bin..."
sudo make install

echo "Installation complete."
echo "You can now bind 'gramfix' to Alt+G in your desktop environment's custom shortcut settings."
