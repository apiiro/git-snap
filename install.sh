#!/bin/bash

# USAGE: ./install.sh
# will install the latest tool executable to your /usr/local/bin

TMPDIR=${TMPDIR:-"/tmp"}
pushd "$TMPDIR"
  mkdir -p gitsnap
  pushd gitsnap
    distro=$(if [[ "`uname -s`" == "Darwin" ]]; then echo "osx"; else echo "linux"; fi)
    curl -s https://api.github.com/repos/apiiro/git-snap/releases/latest | grep "browser_download_url.*$distro.zip" | cut -d : -f 2,3 | tr -d \" | xargs curl -sSL -o gitsnap.zip
    unzip gitsnap.zip
    chmod +x gitsnap-*
    cp -nf gitsnap-* /usr/local/bin/git-snap
  popd
  rm -rf gitsnap
popd
