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
   --shortsha                 support short-sha revision (default: false)
   --help, -h                 show help (default: false)
   --version, -v              print the version (default: false)

```

## Examples

```bash
gitsnap --src /var/git/dc-heacth --rev master --out /tmp/dc-heacth-master
gitsnap --src /var/git/dc-heacth --rev master --out /tmp/dc-heacth-master --include "**/*.java" --exclude "**/test/**"
gitsnap --src /var/git/dc-heacth --rev master --out /tmp/dc-heacth-master --include "**/*.java,pom.xml"
```

## Test and Build

```bash
# run tests:
go test ./...
# run benchmark
go test -tags bench ./...
# build executable
go build -o gitsnap .
```

## Benchmark results

Running on 16 cores (relevant only for gitsnap)

```
+----------------------+-----------------------+------------+-----------------+
|      Repository      |        Action         | Time (sec) |   Performance   |
+----------------------+-----------------------+------------+-----------------+
| EVO-Exchange-BE-2019 | git-archive           |       0.05 | baseline        |
| EVO-Exchange-BE-2019 | git-archive + tar -x  |       0.13 | x2.6            |
| EVO-Exchange-BE-2019 | git-worktree-checkout |      0.096 | x1.9            |
| EVO-Exchange-BE-2019 | gitsnap               |      0.075 | x1.5            |
| EVO-Exchange-BE-2019 | gitsnap (**/*.java)   |      0.032 | x0.64 (faster!) |
| elasticsearch        | git-archive           |       1.86 | baseline        |
| elasticsearch        | git-archive + tar -x  |       7.92 | x4.25           |
| elasticsearch        | git-worktree-checkout |       6.71 | x3.6            |
| elasticsearch        | gitsnap               |       6.31 | x3.4            |
| elasticsearch        | gitsnap (**/*.java)   |        5.3 | x2.85           |
+----------------------+-----------------------+------------+-----------------+
```

Legend:

```bash
git-archive -->
  git archive <commitish> -o <output>
git-archive + tar -x -->
  git archive <commitish> | tar -x -C <output>
git-worktree-checkout -->
  git --work-tree <output> checkout <commitish> -f -q -- ./
```

### Credits

<div>Icons made by <a href="https://www.freepik.com" title="Freepik">Freepik</a> from <a href="https://www.flaticon.com/" title="Flaticon">www.flaticon.com</a></div>
