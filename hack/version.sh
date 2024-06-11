#!/usr/bin/env sh

default_version=v0.0.0

# check that this is a git repo that has at least one reference object
if [ ! -d .git ] || [ -z "$(git rev-list --all)" ] ; then
	echo "$default_version"
	exit
fi

head_tag=$(git tag --contains HEAD)
last_tag=$(git tag --sort version:refname --list | tail --lines 1)
commit=$(git rev-parse HEAD)

if [ -n "$head_tag" ]; then
	echo "$head_tag"
	exit
fi

if [ -z "$last_tag" ]; then
	version="$default_version"
else
	version="$last_tag"
fi

if [ -n "$commit" ]; then
	version="$version-$commit"
fi

if [ -n "$(git status --porcelain)" ]; then
	version="$version-dirty"
fi

echo $version