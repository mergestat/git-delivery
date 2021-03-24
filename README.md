[![TODOs](https://badgen.net/https/api.tickgit.com/badgen/github.com/augmentable-dev/git-delivery?branch=main)](https://www.tickgit.com/browse?repo=github.com/augmentable-dev/git-delivery&branch=main)

# git-delivery

`git-delivery` serves files in git repositories over HTTP.
For instance,

```
curl https://git.delivery/github.com/augmentable-dev/git-delivery/README.md
```

will return the contents of this markdown file (`git.delivery` is a free, publicly running instance of this codebase).
It takes advantage of git [partial clones](https://git-scm.com/docs/partial-clone) to only fetch the requested file from the source repository.
Files are only kept on disk for the duration of the request. 
Currently, this service is meant be run statelessly, passing-thru to an upstream git repo.
If the upstream repository is unavailable, so will requests to the running `git-delivery` instance.

## Configuring

| ENV           | Default                                           | Description                                                                                      |
|---------------|---------------------------------------------------|--------------------------------------------------------------------------------------------------|
| PORT          | `8080`                                            | HTTP port to listen on                                                                           |
| ALLOW_AUTH    | `false`                                           | Whether or not to allow requests with basic auth to pass through to the source repo when cloning |
| HTTP_TIMEOUT  | `30s`                                             | How long before requests are cancelled by the server (timed out)                                 |
| ROOT_REDIRECT | `https://github.com/augmentable-dev/git-delivery` | Where requests to `/` should redirect to                                                         |

## TODO

- [x] Pass through authentication
- [ ] Specify revision in request
- [ ] List contents of directories
- [ ] More consistent error handling
