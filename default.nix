{ pkgs }:
pkgs.buildGoModule rec {
  pname = "tfclean";
  version = "0.0.11";
  src = ./.;
  
  vendorHash = "sha256-T5AvLQk6K5aD9rAJWsoIFPCx6MuRna2cnmY2/S0oBMk=";
  
  subPackages = [ "cmd/tfclean" ];
  
  ldflags = [ "-s" "-w" "-X=main.Version=${version}" ];
  
  meta = with pkgs.lib; {
    description = "A tool for cleaning up Terraform configuration files by automatically removing applied moved, import, and removed blocks";
    homepage = "https://github.com/takaishi/tfclean";
  };
}