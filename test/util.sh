#!/bin/bash

OPTION_STOP_ON_FAIL=false

FAIL(){
    TEST_NAME="$1"
    COMMAND="$2"
    echo "FAILURE: $TEST_NAME ($COMMAND)"
    if $OPTION_STOP_ON_FAIL; then
        exit 1
    fi
}

SUCCEED(){
    echo "OK: $1"
}

TEST(){

    FILE_PATH="$1"
    COMMAND="$2"
    CONTENTS_BEFORE="$3"
    CONTENTS_AFTER="$4"
    TEST_NAME="$5"
    

    echo "$3" > "$FILE_PATH"
    echo $($COMMAND) >> /dev/null
    diff $FILE_PATH <(echo "$CONTENTS_AFTER")
    if (( $? == 1 )); then
        FAIL "$TEST_NAME" "$COMMAND"
    else
        SUCCEED "$TEST_NAME" "$COMMAND"
    fi
}

