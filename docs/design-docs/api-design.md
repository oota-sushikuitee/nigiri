# API Design

## Overview

```bash
$ nigiri build <target> <options>
# build the target under the ~/.nigiri/<target>/<commit-hash> directory
# ~/.nigiri/<target>/<commit-hash> has the following structure:
#   - bin/: contains the built binary
#   - src/: contains the source code
#   - build.json: contains the build information
```

```bash
$ nigiri run <options> <target>
# run the target under the ~/.nigiri/<target>/<commit-hash> directory
```

```bash
$ nigiri remove <target> <commit-hash>
# remove the target under the ~/.nigiri/<target>/<commit-hash> directory
```
