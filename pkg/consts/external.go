package consts

// Labels added to optimized container images
const (
	DSLabelVersion           = "mintoolkit.version"
	DSLabelSourceImage       = "mintoolkit.source.image"
	DSLabelSourceImageID     = "mintoolkit.source.image.id"
	DSLabelSourceImageDigest = "mintoolkit.source.image.digest"
)

// Other constants that external users/consumers will see
const (
	//reverse engineered Dockerfile for the target container image
	ReversedDockerfile        = "Dockerfile.reversed"
	ReversedDockerfileOldName = "Dockerfile.fat" //tmp compat
)
