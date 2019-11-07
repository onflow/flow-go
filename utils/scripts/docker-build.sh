#/bin/sh

### Docker Build Script
### Builds and tags a docker image.
### 1st argument is the path to the Dockerfile to build.
### 2nd argument is the image name.
### Subsequent parameters are tag names that the image should be tagged with.

DOCKERFILE_PATH=$1
IMAGE_NAME=$2

# Build the image and tag with tmp so we can identify it while tagging.
# docker build -f $DOCKERFILE_PATH -t $IMAGE_NAME:tmp .

# For each argument after the first two, tag the image.
ARGC=$ # Total number of arguments
ARGV="$@" # List of all arguments

i=3
while [ "$i" -le "$ARGC" ]; do
  echo $i
  TAG=$ARGV[$i]
  echo "asdf"
  echo $TAG
  if [ -t $TAG ]; then
    echo "Tagging..."
    docker tag $IMAGE_NAME:tmp $IMAGE_NAME:$ARGV[$i]
  fi
done

