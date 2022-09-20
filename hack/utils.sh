#!/bin/bash
# shellcheck disable=SC2181,SC2155,SC2128
#
# Copyright 2022 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#         http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

function if_error_exit {
    ###########################################################################
    # Description:                                                            #
    # Validate if previous command failed and show an error msg (if provided) #
    #                                                                         #
    # Arguments:                                                              #
    #   $1 - error message if not provided, it will just exit                 #
    ###########################################################################
    if [ "$?" != "0" ]; then
        if [ -n "$1" ]; then
            RED="\e[31m"
            ENDCOLOR="\e[0m"
            echo -e "[ ${RED}FAILED${ENDCOLOR} ] ${1}"
        fi
        exit 1
    fi
}

function pass_message {
    ###########################################################################
    # Description:                                                            #
    # show [PASSED] in green and a message as the validation passed.          #
    #                                                                         #
    # Arguments:                                                              #
    #   $1 - message to output                                                #
    ###########################################################################
    if [ -z "${1}" ]; then
        echo "pass_message() requires a message"
        exit 1
    fi
    GREEN="\e[32m"
    ENDCOLOR="\e[0m"
    echo -e "[ ${GREEN}PASSED${ENDCOLOR} ] ${1}"
}

function add_to_path {
    ###########################################################################
    # Description:                                                            #
    # Add directory to path                                                   #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1:  directory                                                      #
    ###########################################################################
    [ $# -eq 1 ]
    if_error_exit "Wrong number of arguments to ${FUNCNAME[0]}"

    local directory="${1}"

    [ -d "${directory}" ]
    if_error_exit "Directory \"${directory}\" does not exist"

    case ":${PATH:-}:" in
        *:${directory}:*) ;;
        *) PATH="${directory}${PATH:+:$PATH}" ;;
    esac
}

command_exists() {
    ###########################################################################
    # Description:                                                            #
    # Checkt if a binary exists                                               #
    #                                                                         #
    # Arguments:                                                              #
    #   arg1: binary name                                                     #
    ###########################################################################
    cmd="$1"
    command -v "${cmd}" >/dev/null 2>&1
}

function setup_j2() {
    ###########################################################################
    # Description:                                                            #
    # Install j2 binary                                                       #
    ###########################################################################
    if ! command_exists j2 ; then
	# Add path to pip --user installation directory
	if [ -d "${HOME}"/.local/bin ] ; then
           add_to_path "${HOME}"/.local/bin
	fi
    fi

    if ! command_exists j2 ; then
	if command_exists pip ; then
            echo "'j2' not found, installing with 'pip'"
            pip install wheel --user
            pip freeze | grep j2cli || pip install j2cli[yaml] --user
            if_error_exit "cannot download j2"
	elif command_exists python3 ; then
            echo "'j2' not found, installing with 'python3 -m pip'"
            python3 -m pip install wheel --user
            python3 -m pip freeze | grep j2cli || python3 -m pip install j2cli[yaml] --user
            if_error_exit "cannot download j2"
	elif command_exists python2 ; then
            echo "'j2' not found, installing with 'python2 -m pip'"
            python2 -m pip install wheel --user
            python2 -m pip freeze | grep j2cli || python2 -m pip install j2cli[yaml] --user
            if_error_exit "cannot download j2"
	elif command_exists python ; then
            echo "'j2' not found, installing with 'python -m pip'"
            python -m pip install wheel --user
            python -m pip freeze | grep j2cli || python -m pip install j2cli[yaml] --user
            if_error_exit "cannot download j2"
	else
	    echo "cannot find a tool or python that can downlad j2"
	    exit 1
	fi
        add_to_path "${HOME}"/.local/bin
	pass_message "The tool j2 is installed."
    fi
}

