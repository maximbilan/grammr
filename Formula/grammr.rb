# typed: false
# frozen_string_literal: true

# To update this formula for a new version:
# 1. Update the tag version below (e.g., "v1.0.3" -> "v1.0.4")
# 2. Commit and push the changes
# 3. Create a git tag: git tag v1.0.4 && git push origin v1.0.4
# 4. Update the install.sh script version if needed

class Grammr < Formula
  desc "Lightning-fast AI grammar checker in your terminal"
  homepage "https://github.com/maximbilan/grammr"
  url "https://github.com/maximbilan/grammr.git",
      tag:      "v1.0.4"
  license "MIT"
  head "https://github.com/maximbilan/grammr.git", branch: "main"

  depends_on "go" => :build

  def install
    system "go", "build", "-o", bin/"grammr", "."
  end

  test do
    # Test that the binary works
    system "#{bin}/grammr", "config", "get", "model"
  end
end
