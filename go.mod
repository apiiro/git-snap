module gitsnap

go 1.14

require (
	github.com/avast/retry-go v3.0.0+incompatible
	github.com/go-git/go-git/v5 v5.3.0
	github.com/gobwas/glob v0.2.3
	github.com/shomali11/parallelizer v0.0.0-20210324142433-ae8e5504ab47
	github.com/stretchr/testify v1.4.0
	github.com/urfave/cli/v2 v2.3.0
)

replace github.com/go-git/go-git/v5 v5.3.0 => github.com/apiiro/go-git/v5 v5.3.1
