# Plattentests.de - Highlights der Woche

[![Build and deploy to Azure function](https://github.com/jetzlstorfer/plattentests-go/actions/workflows/deploy-functions.yml/badge.svg)](https://github.com/jetzlstorfer/plattentests-go/actions/workflows/deploy-functions.yml)
[![Build and deploy to Azure Container Apps](https://github.com/jetzlstorfer/plattentests-go/actions/workflows/deploy-aca.yml/badge.svg)](https://github.com/jetzlstorfer/plattentests-go/actions/workflows/deploy-aca.yml)
[![CodeQL](https://github.com/jetzlstorfer/plattentests-go/actions/workflows/codeql.yml/badge.svg)](https://github.com/jetzlstorfer/plattentests-go/actions/workflows/codeql.yml)
[![Dependency Review](https://github.com/jetzlstorfer/plattentests-go/actions/workflows/dependency-review.yml/badge.svg)](https://github.com/jetzlstorfer/plattentests-go/actions/workflows/dependency-review.yml)

👨‍💻👩‍💻 **Please note that this project currently serves multiple purposes** 👨‍💻👩‍💻

1. The original purpose of generating a [Spotify playlist](https://open.spotify.com/playlist/2Bc5TRdMTj6OHwt32x5T6Y?si=c7cf976d4d124bef) that lists all "highlights" of the week of my personal favourite music website [Plattentests.de](https://plattentests.de).
1. The purpose of getting to know more about serverless and Azure functions
1. A playground for features like
   - Codespaces & devcontainers,
   - GitHub actions,
   - GitHub co-pilot and other features of GitHub.

Therefore, some commit messages might not be useful at the moment :)

# Architecture

```mermaid
C4Context
    title System Context diagram for Internet Banking System
    Enterprise_Boundary(b0, "BankBoundary0") {
    Person(customerA, "Banking Customer A", "A customer of the bank, with personal bank accounts.")
    Person(customerB, "Banking Customer B")
    Person_Ext(customerC, "Banking Customer C", "desc")

    Person(customerD, "Banking Customer D", "A customer of the bank, <br/> with personal bank accounts.")

    System(SystemAA, "Internet Banking System", "Allows customers to view information about their bank accounts, and make payments.")

    Enterprise_Boundary(b1, "BankBoundary") {

        SystemDb_Ext(SystemE, "Mainframe Banking System", "Stores all of the core banking information about customers, accounts, transactions, etc.")

        System_Boundary(b2, "BankBoundary2") {
        System(SystemA, "Banking System A")
        System(SystemB, "Banking System B", "A system of the bank, with personal bank accounts. next line.")
        }

        System_Ext(SystemC, "E-mail system", "The internal Microsoft Exchange e-mail system.")
        SystemDb(SystemD, "Banking System D Database", "A system of the bank, with personal bank accounts.")

        Boundary(b3, "BankBoundary3", "boundary") {
        SystemQueue(SystemF, "Banking System F Queue", "A system of the bank.")
        SystemQueue_Ext(SystemG, "Banking System G Queue", "A system of the bank, with personal bank accounts.")
        }
    }
    }

    BiRel(customerA, SystemAA, "Uses")
    BiRel(SystemAA, SystemE, "Uses")
    Rel(SystemAA, SystemC, "Sends e-mails", "SMTP")
    Rel(SystemC, customerA, "Sends e-mails to")

    UpdateElementStyle(customerA, $fontColor="red", $bgColor="grey", $borderColor="red")
    UpdateRelStyle(customerA, SystemAA, $textColor="blue", $lineColor="blue", $offsetX="5")
    UpdateRelStyle(SystemAA, SystemE, $textColor="blue", $lineColor="blue", $offsetY="-10")
    UpdateRelStyle(SystemAA, SystemC, $textColor="blue", $lineColor="blue", $offsetY="-40", $offsetX="-50")
    UpdateRelStyle(SystemC, customerA, $textColor="red", $lineColor="red", $offsetX="-50", $offsetY="20")

    UpdateLayoutConfig($c4ShapeInRow="3", $c4BoundaryInRow="1")
```



# Usage


💡 For your own convenience, make use of Codespaces or run it locally as devcontainer.

There is a `Makefile` with multiple targets to be used. 
⚠️ Make sure you have the proper `ENV` variables set in a `.env` file.

- To create a token and store it in Azure:
    ```
    make token
    ```

- To run the project locally as Go binary:
    ```
    make run
    ```

- To run the project locally as a function:
    ```
    make run-function
    ```

- To run the web-frontend of the project (located in `./webui`):
    ```
    make web
    ```


## As Docker container

You can also run the project as a Docker container.

- Build and run the container:
    ```
    docker build -t plattentests-go .
    docker run -p 8080:8080 plattentests-go
    ```
