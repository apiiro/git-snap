# git-snap

Tool to create a git revision snapshot for an existing repository clone.

![icon](git-snap.png)

```
NAME:
   git-snap - 1.6.0 - Create a git revision snapshot for an existing repository clone. Symbolic link files will be omitted.

USAGE:
   git-snap --src value --rev value --out value         [optional flags]

OPTIONS:
   --src value, -s value      path to existing git clone as source directory, may contain no more than .git directory, current git state doesn't affect the command
   --rev value, -r value      commit-ish Revision
   --out value, -o value      output directory. will be created if does not exist
   --include value, -i value  patterns of file paths to include, comma delimited, may contain any glob pattern
   --exclude value, -e value  patterns of file paths to exclude, comma delimited, may contain any glob pattern
   --verbose, --vv            verbose logging (default: false)
   --text-only                include only text files (default: false)
   --hash-markers             create also hint files mirroring the hash of original files at <path>.hash (default: false)
   --ignore-case              ignore case when checking path against inclusion patterns (default: false)
   --max-size value           maximal file size, in MB (default: 6)
   --no-double-check          disable files discrepancy double check (default: false)
   --include-noise-dirs       don't filter out noisy directory names in paths (bin, node_modules etc) (default: false)
   --help, -h                 show help (default: false)
   --version, -v              print the version (default: false)

EXIT CODES:
  0   Success
  201  Clone path is invalid (fs-wise)
  202  Clone path is invalid (git-wise)
  203  Output path is invalid
  204  Short sha is not supported
  205  Provided revision could not be found
  206 Double check for files discrepancy failed
  1  Any other error
```

## Examples

```bash
git snap --src /var/shared/git/dc-heacth --rev master --out /tmp/dc-heacth-master
git snap --src /var/shared/git/dc-heacth --rev master --out /tmp/dc-heacth-master --include "**/*.java" --exclude "**/test/**"
git snap --src /var/shared/git/dc-heacth --rev master --out /tmp/dc-heacth-master --include "**/*.java,pom.xml"
```

## Install

```bash
curl -s https://raw.githubusercontent.com/apiiro/git-snap/main/install.sh | sudo bash
# or for a specific version:
curl -s https://raw.githubusercontent.com/apiiro/git-snap/main/install.sh | sudo bash -s 1.4
```

If that doesn't work, try:
```bash
curl -s https://raw.githubusercontent.com/apiiro/git-snap/main/install.sh -o install.sh
sudo bash install.sh
```

then run with either:

```bash
git-snap -h
git snap -h
```

## Test and Build

```bash
# run tests:
make test
# run benchmark
make benchmark
# build binaries and run whole ci flow
make
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
