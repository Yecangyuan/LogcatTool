class Logcatool < Formula
  desc "终端版 Android Logcat 查看器，基于 Bubble Tea 构建"
  homepage "https://github.com/simley/logcatool"
  version "0.1.0"

  # macOS Intel
  if OS.mac? && Hardware::CPU.intel?
    url "https://github.com/simley/logcatool/releases/download/v0.1.0/logcatool_0.1.0_darwin_amd64.tar.gz"
    sha256 "YOUR_SHA256_HERE"
  end

  # macOS ARM (Apple Silicon)
  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/simley/logcatool/releases/download/v0.1.0/logcatool_0.1.0_darwin_arm64.tar.gz"
    sha256 "YOUR_SHA256_HERE"
  end

  # Linux AMD64
  if OS.linux? && Hardware::CPU.intel?
    url "https://github.com/simley/logcatool/releases/download/v0.1.0/logcatool_0.1.0_linux_amd64.tar.gz"
    sha256 "YOUR_SHA256_HERE"
  end

  # Linux ARM64
  if OS.linux? && Hardware::CPU.arm?
    url "https://github.com/simley/logcatool/releases/download/v0.1.0/logcatool_0.1.0_linux_arm64.tar.gz"
    sha256 "YOUR_SHA256_HERE"
  end

  depends_on "adb" => :optional

  def install
    bin.install "logcatool"
  end

  test do
    system "#{bin}/logcatool", "-v"
  end
end
