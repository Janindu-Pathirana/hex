class Hex < Formula
  desc "CLI tool for analyzing companies on the Sri Lanka stock market"
  homepage "https://github.com/Janindu-Pathirana/hex"
  head "https://github.com/Janindu-Pathirana/hex.git", branch: "main"

  depends_on "go" => :build

  def install
    ldflags = "-X github.com/Janindu-Pathirana/hex/cmd.Version=#{version}"
    system "go", "build", *std_go_args(ldflags:), "-o", bin/"hex", "."
  end

  test do
    assert_match "hex", shell_output("#{bin}/hex --help")
  end
end
