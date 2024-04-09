name: Read Issue and Write to File

on:
  issues:
    types: [opened, edited]

jobs:
  write_to_file:
    runs-on: ubuntu-latest

    steps:
    - name: Checkout code
      uses: actions/checkout@v2

    - name: Read issue and write to file
      uses: actions/github-script@v4
      with:
        script: |
          const fs = require('fs');
          const issue_number = context.issue.number;
          const owner = context.repo.owner;
          const repo = context.repo.repo;
          
          const issue = await github.rest.issues.get({
            owner: owner,
            repo: repo,
            issue_number: issue_number,
          });

          fs.writeFileSync('./ISSUE_CONTENT.md', issue.data.body);
          
    - name: Commit and push if it changed
      run: |
        git config --global user.name 'Automated'
        git config --global user.email 'actions@users.noreply.github.com'
        git add -A
        git diff --quiet && git diff --staged --quiet || git commit -m 'Updated ISSUE_CONTENT.md'
        git push
