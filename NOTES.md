
Backend should be git repo. It does the work already, so why not leverage it. 

Client can then sync straight to git server, and can sync to a personal git repo if needed (Github, eg)

For own data store, use EFS with a git server, and use standard unix authentication primitives. 
IE: git access is via ssh, with keyswap. 

Small shell layer to manage creating, enabling/disabling and removing clients.


## Additional infra

### Basic flow*
```
[client] <-----Git-----> [EC2 Endpoint] <-----SQL------> [AWS RDS user store]
                                      + <--Filesystem--> [AWS EFS data store]
```

### Key Questions on Infra
Q: How is the server code built and deployed to an EC2 instance?

- S3 bucket for Terraform info
- S3 bucket for AMI images for EC2 endpoint
- CI pipeline for AMI images
- Reactive update of EC2 images
- CD with AWS CodePipeline
- Website (S3)
- Installer archive (S3)
- Elastic beanstalk with efs (https://docs.aws.amazon.com/elasticbeanstalk/latest/dg/services-efs.html) and Go for code


### Infra
EC2 instance - stateless
EFS for backing store
Terraform for provisioning

### Identity management
AWS RDS Id/Email/Password/Billing/Notes (Aurora)

### Git servers
https://github.com/stackdot/NodeJS-Git-Server - Deprecated library use (2011)
https://github.com/AaronO/go-git-http - Go Git server library (2016) with delegated auth
https://github.com/sosedoff/gitkit - Go Git server library (2018, Jan) with delegated Auth
https://github.com/src-d/go-git - Go Git implementation (2018, recent)
https://github.com/gogs/gogs - Complete server solution (2018, recent)
https://github.com/gogs/git-module - Shell over git CLI (2018, recent)
https://github.com/libgit2/git2go libgit2 Go bindings (libgit2 is complete in term of functionality, including merges, etc...)

# Implementation plan

- [ ] Develop the go-git client against github private repo 

- [ ] Get go-git client to talk to a standard git repo over ssh
- [ ] Get git to clone from an ssh gitkit server
- [ ] Get git to clone & push from an SSH server with user authentication from staticly configured public key
- [ ] Get go-git to talk to a gitkit server with custom ssh keys
- [ ] Get the gitkit server to get the ssh keys from an SQL database

# Questions
Q: Auth on SSH server besides Keys? - Not needed. Just don't provide key in the callback if delinquent.
Q: Stop users reading each-other's repos? - Unique, high entropy key / token

