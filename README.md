# Plattentests.de - Highlights der Woche

:warning: readme has to be updated!

:warning: Please note that this project currently serves as a playground and thus, serves multiple purposes, not only the original purpose of providing a spotify playlist. it is currently also used as playground for codespaces, co-pilot and other features of github.
therefore, a lot of commit messages might not be useful at the moment :)

This program collects the current records of the week from http://plattentests.de and updates a playlist with all highlights. Therefore we are going to use Azure functions. Please note the original version of this program was using AWS lambda (see branch)

# Usage

## first token export currently not working. you need to have a valid token for it to run successfully.

- Create the token first. Make sure you have the ENV variables set: `
  ```
  export TOKEN_FILE=
  export SPOTIFY_ID=
  export SPOTIFY_SECRET=
  ```
  Then run the file:
  ```
  go run cmd/token.go
  ```

## needs rework!

- Run locally with the [Azure functions core tools](https://docs.microsoft.com/en-us/azure/azure-functions/functions-run-local?tabs=v4%2Cwindows%2Ccsharp%2Cportal%2Cbash)
  ```
  go build .\cmd\crawler.go
  func start
  ```

- Create Lambda function
  ```
  make upload
  ```

- Update Lambda function if source code has changed
  ```
  make update
  ```

## Run it locally on a Windows machine

To run it on Windows, I have the [Git bash](https://gitforwindows.org/) installed and run it from there. 


## Configure Lambda function
To set up the Lambda function to run on a predefined schedule, configure it in your AWS console.
