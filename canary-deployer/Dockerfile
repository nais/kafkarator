FROM docker.pkg.github.com/nais/deploy/deploy:latest
ADD deployer.sh /canary/deployer.sh
ADD canary.yml /canary/resource.yml
ENV RESOURCE=/canary/resource.yml
CMD ["/canary/deployer.sh"]
