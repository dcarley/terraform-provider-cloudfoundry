#!/usr/bin/env bash
REPO_NAME="terraform-provider-cloudfoundry"
NAME="terraform-provider-cloudfoundry"
OS=""
OWNER="orange-cloudfoundry"
: "${TMPDIR:=${TMP:-$(CDPATH=/var:/; cd -P tmp)}}"
cd -- "${TMPDIR:?NO TEMP DIRECTORY FOUND!}" || exit
cd -

which terraform
if [[ $? != 0 ]]; then
    echo "you must have terraform installed"
fi
tf_version=$(terraform --version | awk '{print $2}')
tf_version=${tf_version:1:3}

echo "Installing ${NAME}..."
if [[ "$OSTYPE" == "linux-gnu" ]]; then
    OS="linux"
elif [[ "$OSTYPE" == "darwin"* ]]; then
    OS="darwin"
elif [[ "$OSTYPE" == "cygwin" ]]; then
    OS="windows"
elif [[ "$OSTYPE" == "msys" ]]; then
    OS="windows"
elif [[ "$OSTYPE" == "win32" ]]; then
    OS="windows"
else
    echo "Os not supported by install script"
    exit 1
fi

ARCHNUM=`getconf LONG_BIT`
ARCH=""
CPUINFO=`uname -m`
if [[ "$ARCHNUM" == "32" ]]; then
    ARCH="386"
else
    ARCH="amd64"
fi
if [[ "$CPUINFO" == "arm"* ]]; then
    ARCH="arm"
fi
FILENAME="${NAME}_${OS}_${ARCH}_${tf_version}"
if [[ "$OS" == "windows" ]]; then
    FILENAME="${FILENAME}.exe"
fi

VERSION=$(curl -s https://api.github.com/repos/${OWNER}/${REPO_NAME}/releases/latest | grep tag_name | head -n 1 | cut -d '"' -f 4)

LINK="https://github.com/${OWNER}/${REPO_NAME}/releases/download/${VERSION}/${FILENAME}"
if [[ "$OS" == "windows" ]]; then
    FILEOUTPUT="${FILENAME}"
else
    FILEOUTPUT="${TMPDIR}/${FILENAME}"
fi
RESPONSE=200
if hash curl 2>/dev/null; then
    RESPONSE=$(curl --write-out %{http_code} -L -o "${FILEOUTPUT}" "$LINK")
else
    wget -o "${FILEOUTPUT}" "$LINK"
    RESPONSE=$?
fi

if [ "$RESPONSE" != "200" ] && [ "$RESPONSE" != "0" ]; then
    echo "File ${LINK} not found, so it can't be downloaded."
    rm "$FILEOUTPUT"
    exit 1
fi

chmod +x "$FILEOUTPUT"
mkdir -p ~/.terraform.d/providers/
if [[ "$OS" == "windows" ]]; then
    mv "$FILEOUTPUT" "~/.terraform.d/providers/${NAME}"
else
    mv "$FILEOUTPUT" "~/.terraform.d/providers/${NAME}"
fi


grep -Fxq "providers {" ~/.terraformrc
if [[ $? != 0 ]]; then
    cat <<EOF >> .terraformrc
providers {
    cloudfoundry = "~/.terraform.d/providers/terraform-provider-cloudfoundry"
}
EOF
else
    awk '/providers {/ { print; print "cloudfoundry = \"~/.terraform.d/providers/terraform-provider-cloudfoundry\""; next }1' ~/.terraformrc > /tmp/.terraformrc
    mv /tmp/.terraformrc ~/
fi

echo "${NAME} has been installed."