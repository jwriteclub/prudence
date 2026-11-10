<!-- Copyright 2026 Christopher O'Connell -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# Pinned standard resources

These unmodified files are the `tla2sany/StandardModules/*.tla` entries of
the stock oracle JAR SHA-256
`171c46c1ec562ff1893a709916ab2616865bb3a635529ec7da22c8699ae9c35a`.
Each entry was compared byte-for-byte with the corresponding source at TLA+
revision `dc6a944429fe3e26104d38f9ff792d2d5d83f307`. The files are under the
TLA+ MIT license in `licenses/TLAplus.txt`; their original notices remain.
The following hashes were generated from the JAR entries:

```
a2b668a8cf375b9ef9f2c14fdba742137ab8722ade0f41f11965c67acba76c39  Bags.tla
d94cb545256539bf782fc1914842dfe8c19c1d3d4e48b8327dd0c47b74922e1e  FiniteSets.tla
071da69e5eea383d361ef308ba12ecc931bf968fcea98175c3bca419bbd8cbf2  Integers.tla
52e38eeed89c35ef9d65ac7f6e884a8ae761c7302a52c35c0f0658035ddca0b8  Json.tla
73f2a68412039aa86f37ad08f0d0fef50fa8327d0ad8806f515548c75e9d9d84  Naturals.tla
998209fcb33bac3c9b4d6851dc5df5ca83d33722201cc5280b51a1f58fffaebf  Randomization.tla
9052ccc8a150c93ae13a4d136da7b83dc68d1d8e9a0eee6f787db02b755f6682  RealTime.tla
33cc699c0139b39fcee903d4dde255b64a0b430291630aa4825c6dd32b2f2fa2  Reals.tla
f613cffe14133769f0972b6f6e0139aaf0773004a51bac88ada382426e3eb7f4  Sequences.tla
944ece590eec81ace059881c2a62064c06041efa81539a788cd40fa7d1da41ce  TLC.tla
6c3e7e54cf1b8a4d2d1c7a5fd088f06452edce68ec28d8be99392cc0037a5399  TLCExt.tla
9deac1facb0b71809714229dace116da75f80bf4c65b707ff40026f380c7804a  Toolbox.tla
c2f4b13446298c026decf0b3867134d2a176355344db05af7270e31981f0c8bc  _DotTrace.tla
4e93f1de053afb59cbeb5ae6f0d71685acf3530c840ac4811f54e86100195742  _JsonTrace.tla
3c360af74043d85a0399376aa2b1553ca9d230d06a34bd46cbae1f1e02109a13  _Possible.tla
ebc408fd614b832d2f8eab1a4dae7ae67c6d5a7958e0233c7da702c1a0e29705  _TLAPlusCounterExample.tla
4cb2f1e02109cee0053f59e72bdfb59edab12a30cddf4bc1765927288db2d134  _TLCActionTrace.tla
ee5b74e29dcd8d64f2a8840c293018a2007371aab14ea0eaa7c542de8f97b4aa  _TLCTESpec.tla
864407a426e7e438af3652a5db92912bfdf682cf97912c6bdc91d87cb94a5007  _TLCTrace.tla
4f05472f833eadec96a2ec8cb2f1331ce8902bf08c800b80407d2980809e6d09  _TLCTracePlain.tla
```

At that revision `SimpleFilenameToStream` searches the user directory or CWD,
then configured filesystem paths, then the `tla2sany/StandardModules/`
classpath prefix, then the bare classpath. `FileUtil` supplies the `.tla`
filename. Prudence delegates user-source lookup to the caller and only embeds
the named standard namespace; it does not search a runtime classpath or disk.
A successful caller result wins and keeps its declared origin. Only a wrapped
or direct missing sentinel allows fallback; operational errors stop resolution.
The namespace inventory is not the separate nine-name standard classifier.
