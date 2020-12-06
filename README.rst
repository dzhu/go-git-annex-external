#######################
 go-git-annex-external
#######################

go-git-annex-external is a library for creating git-annex_ external `special
remotes`_ and external backends_ using Go. Clients of the library are insulated
from the specifics of the communication protocols and only need to implement
individual operations (fulfilling particular Go interfaces) to produce external
remotes and backends.

Both protocols as they stand at the time of writing are fully supported,
including (for external special remotes) the info extension, the `async
extension`_, and the `simple export interface`_.

**************************
 External special remotes
**************************

-  `API documentation <remote-docs_>`_
-  `protocol documentation <remote-protocol_>`_

See the ``cmd/git-annex-remote-local`` subdirectory for a functioning example.
Here's a minimal compilable skeleton:

.. code:: go

   package main

   import "github.com/dzhu/go-git-annex-external/remote"

   type minimalRemote struct{}

   func (*minimalRemote) Init(a remote.Annex) error                        { return nil }
   func (*minimalRemote) Prepare(a remote.Annex) error                     { return nil }
   func (*minimalRemote) Store(a remote.Annex, key, file string) error     { return nil }
   func (*minimalRemote) Retrieve(a remote.Annex, key, file string) error  { return nil }
   func (*minimalRemote) Present(a remote.Annex, key string) (bool, error) { return false, nil }
   func (*minimalRemote) Remove(a remote.Annex, key string) error          { return nil }

   func main() {
       remote.Run(&minimalRemote{})
   }

*******************
 External backends
*******************

-  `API documentation <backend-docs_>`_
-  `protocol documentation <backend-protocol_>`_

See the ``cmd/git-annex-backend-XSHORTHASH`` subdirectory for a functioning
example. Here's a minimal compilable skeleton:

.. code:: go

   package main

   import "github.com/dzhu/go-git-annex-external/backend"

   type minimalBackend struct{}

   func (*minimalBackend) GenKey(a backend.Annex, file string) (string, bool, error) {
       return "", false, nil
   }
   func (*minimalBackend) IsStable(a backend.Annex) bool { return false }

   func main() {
       backend.Run(&minimalBackend{})
   }

.. _api documentation: https://pkg.go.dev/github.com/dzhu/go-git-annex-external

.. _async extension: https://git-annex.branchable.com/design/external_special_remote_protocol/async_appendix/

.. _backend-docs: https://pkg.go.dev/github.com/dzhu/go-git-annex-external/backend

.. _backend-protocol: https://git-annex.branchable.com/design/external_backend_protocol/

.. _backends: https://git-annex.branchable.com/backends/

.. _git-annex: https://git-annex.branchable.com

.. _remote-docs: https://pkg.go.dev/github.com/dzhu/go-git-annex-external/remote

.. _remote-protocol: https://git-annex.branchable.com/design/external_special_remote_protocol/

.. _simple export interface: https://git-annex.branchable.com/design/external_special_remote_protocol/export_and_import_appendix/

.. _special remotes: https://git-annex.branchable.com/special_remotes/
