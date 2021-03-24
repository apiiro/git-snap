# git-snap

Tool to create a git revision snapshot for an existing repository clone.

![icon](git-snap.png)

```
NAME:
   git-snap - 1.0 - Create a git revision snapshot for an existing repository clone.
                    Symbolic link files will be omitted.

USAGE:
   git-snap --src value --rev value --out value [--include value] [--exclude value] [--shortsha value]

OPTIONS:
   --src value, -s value      path to existing git clone as source directory, may contain no more than .git directory, current git state doesn't affect the command
   --rev value, -r value      commit-ish Revision
   --out value, -o value      output directory. will be created if does not exist
   --include value, -i value  patterns of file paths to include, comma delimited, may contain any glob pattern
   --exclude value, -e value  patterns of file paths to exclude, comma delimited, may contain any glob pattern
   --shortsha                 support short-sha Revision (default: false)
   --help, -h                 show help (default: false)
   --version, -v              print the version (default: false)

```

## Examples

```bash
git-snap --src /var/git/dc-heacth --rev master --out /tmp/dc-heacth-master
git-snap --src /var/git/dc-heacth --rev master --out /tmp/dc-heacth-master --include "**/*.java" --exclude "**/test/**"
git-snap --src /var/git/dc-heacth --rev master --out /tmp/dc-heacth-master --include "**/*.java,pom.xml"
```

## Test and Build

```bash
# run tests:
go test ./...
# run benchmark
go test -tags bench ./...
# build executable
go build -o git-snap .
```
