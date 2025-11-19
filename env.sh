python3 -m venv venv

source ./venv/bin/activate
export PATH=`pwd`/venv/bin:$PATH

pip install mkdocs mkdocs-glightbox mkdocs-material mkdocs-git-revision-date-localized-plugin mkdocs-include-markdown-plugin mkdocs-rss-plugin
