FROM cloudfoundry/cflinuxfs2
ADD stager install_docker /
RUN chown -R vcap:vcap /tmp
RUN /install_docker
RUN mkdir -p /cache
RUN chown -R vcap:vcap /cache
CMD /stager
