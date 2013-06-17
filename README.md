EPUB3 for LinuxFr.org contents
==============================

This daemon creates on the fly epub3 from a content on LinuxFr.org and its
comments.


How to use it?
--------------

[Install Go 1](http://golang.org/doc/install) and don't forget to set `$GOPATH`

    # aptitude install libonig-dev libxml2-dev pkg-config
    $ go get -u github.com/nono/epub-LinuxFr.org
    $ epub-LinuxFr.org [-addr addr] [-l logs]

And, to display the help:

    $ epub-LinuxFr.org -h


See also
--------

* [Git repository](http://github.com/nono/epub-LinuxFr.org)


Copyright
---------

The code is licensed as GNU AGPLv3. See the LICENSE file for the full license.

♡2013 by Bruno Michel. Copying is an act of love. Please copy and share.
