echo -e "[dailybuild_factory]\nname=dailybuild_factory\nbaseurl=http://119.3.219.20:82/openEuler:/Factory/standard_x86_64/\nenabled=1\ngpgcheck=0\n\n[dailybuild_mainline]\nname=dailybuild_mainline\nbaseurl=http://119.3.219.20:82/openEuler:/Mainline/standard_x86_64/\nenabled=1\ngpgcheck=0" > /etc/yum.repos.d/openEuler.repo
yum clean all
yum makecache
yum -y install openEuler-Advisor
git config --global user.name "review_tool"
git config --global user.email "review_tool@example.com"
