#! /bin/bash

site=$1
username=$2
password=$3

if test -z $site || test -z $username || test -z $password; then
  echo "data has empty!"
  exit 1
fi

if ! test -f ~/.gitconfig; then
  touch ~/.gitconfig
fi

hasStore=`cat ~/.gitconfig | grep "helper = store"`

if test -z "$hasStore"; then
  echo "will config git helper!"

  cat << EOF >> ~/.gitconfig
[credential]
  helper = store
EOF
fi

hasStoreUsername=`cat ~/.gitconfig | grep "credential \"$site\""`

if test -z "$hasStoreUsername"; then
  echo "will config git credential username!"

  cat << EOF >> ~/.gitconfig
[credential "$site"]
	username = $username
EOF
fi

if ! test -f ~/.git-credentials; then
  touch ~/.git-credentials
fi

_site=`echo $site | sed "s/^\(https*:\/\/\)/\1$username:$password@/g"`
hasStorePassword=`cat ~/.git-credentials | grep $_site`

if test -z "$hasStorePassword"; then
  echo "will config git credential password!"
  cat << EOF >> ~/.git-credentials
$_site
EOF
fi

exit 0