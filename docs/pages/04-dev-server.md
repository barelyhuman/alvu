# Development Server

The tool comes with it's own dev server (since v0.2.4), and can be used to run a
simple file server for the output folder.

It's as simple as running the following command

```sh
$ alvu --serve
# short for
$ alvu --serve --port=3000
```

## Live Reload

<small>Added in `v0.2.9`</small>

There's no changes that you need to do, just upgrade your installation to
`v0.2.9`

#### What's to be expected

- Will reload on changes from the directories `pages`, `public` , or if you
  changed them with flags then the respective paths will be watched instead

- The rebuilding process is atomic and will recompile a singular file if that's
  all that's changed instead of compiling the whole folder. This is only true
  for files in the `pages` directory, if any changes were made in `public`
  directory then the whole alvu setup will rebuild itself again.

#### Caveats

- `./hooks` are not watched, this is because hooks have their own state and
  involve a VM, the current structure of alvu stops us from doing this but
  should be done soon without increasing complexity or size of alvu itself.
