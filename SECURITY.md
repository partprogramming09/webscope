# Security

**Reader:** people who found a vulnerability

Please do not open a public issue. Report it privately through GitHub: the **Security** tab of this repository, then **Report a vulnerability**. Include what you ran, what happened, and the version (`webscope` prints it on start, and `CHANGELOG.md` lists them).

Only the latest release is supported.

What counts most here: anything that lets `query` or `code` write to the database, lets `page`'s local listener answer an origin other than `WEBSCOPE_APP_URL`, or lets `code` read outside its folders or follow a symbolic link.
