
## Inspiration & Acknowledgments

https://www.thorsten-hans.com/azure-functions-with-go/ 


## Setting up codespace

## 

## logging in to azure

make sure to use `--use-device-code` if the connection to localhost does break your login. also, you might need to use a specific tenant during login already.
```
az login --tenant eae05f48-5c26-49ee-9b75-c75068e589c0 --use-device-code
```

az functionapp create -n plattentests-go -g rg-plattentests-go --consumption-plan-location germanywestcentral --os-type linux --runtime custom --functions-version 3 --storage-account safplattentests


func azure functionapp publish plattentests-go


## makefile

# HOW TO ACCESS BLOG STORAGE

https://www.eventslooped.com/posts/use-golang-to-upload-files-to-azure-blob-storage/


# NEW SECTION

how to access spotify api


