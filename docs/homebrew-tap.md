# Homebrew Tap Setup

`qp` can be installed via Homebrew from a tap repository:

```bash
brew install neural-chilli/tap/qp
```

This repo does not host the tap itself; Homebrew formulas should live in a separate tap repo (for example `neural-chilli/homebrew-tap`).

## Create The Tap Repo

1. Create `https://github.com/neural-chilli/homebrew-tap`.
2. Add a formula file at `Formula/qp.rb`.
3. Push the formula update after each tagged release.

## Formula Template

Update version and SHA256 values per release asset.

```ruby
class Qp < Formula
  desc "Quickly Please: local-first task runner and workflow runtime"
  homepage "https://github.com/neural-chilli/qp"
  version "0.1.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/neural-chilli/qp/releases/download/v#{version}/qp_v#{version}_darwin_arm64.tar.gz"
      sha256 "<darwin-arm64-sha256>"
    else
      url "https://github.com/neural-chilli/qp/releases/download/v#{version}/qp_v#{version}_darwin_amd64.tar.gz"
      sha256 "<darwin-amd64-sha256>"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/neural-chilli/qp/releases/download/v#{version}/qp_v#{version}_linux_arm64.tar.gz"
      sha256 "<linux-arm64-sha256>"
    else
      url "https://github.com/neural-chilli/qp/releases/download/v#{version}/qp_v#{version}_linux_amd64.tar.gz"
      sha256 "<linux-amd64-sha256>"
    end
  end

  def install
    bin.install "qp"
  end

  test do
    assert_match "qp", shell_output("#{bin}/qp version")
  end
end
```

## Updating For A Release

1. Tag and publish the release in `neural-chilli/qp`.
2. Download `dist/checksums.txt` from the release.
3. Update `Formula/qp.rb` URLs + SHA256 values.
4. Commit and push in the tap repo.
5. Verify install:

```bash
brew tap neural-chilli/tap
brew install neural-chilli/tap/qp
qp version
```
