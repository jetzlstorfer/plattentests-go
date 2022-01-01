# Plattentests.de - Highlights der Woche

:warning: readme has to be updated

This programm collects the current records of the week from http://plattentests.de and updates a playlist with all highlights.

🙏 Authentication is based on https://www.zachjohnsondev.com/posts/managing-spotify-library/ (kudos!)

# Usage

- Create the token first
  ```
  make token
  ```

- Run locally
  ```
  make run
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
