# Plattentests.de - Highlights der Woche

:warning: readme has to be updated

This programm collects the current records of the week from http://plattentests.de and updates a playlist with all highlights.

authenntication is based on 
https://www.zachjohnsondev.com/posts/managing-spotify-library/

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

