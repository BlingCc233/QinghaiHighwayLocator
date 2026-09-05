#@category OMAP

from ghidra.app.decompiler import DecompInterface

targets = [
    "140841fd0", # OpenDBToMemory path used by oobj.odb preload
    "1408df310", # encrypted SQLite key/open helper used by OpenDBToMemory
    "1408df190", # SQLite pager codec callback
    "14231bf60", # key material derivation helper
    "14231bfe0", # page cipher implementation
    "14231ca70", # cipher key schedule setup
    "14231c440", # page/offset keystream setup
    "14231c630", # write-side page finalizer
    "14231c810", # IDEA block transform used by the CFB state
]

decompiler = DecompInterface()
decompiler.openProgram(currentProgram)
for raw in targets:
    function = getFunctionAt(toAddr(long(raw, 16)))
    print("BEGIN {} {}".format(raw, function))
    result = decompiler.decompileFunction(function, 30, monitor)
    if not result.decompileCompleted():
        print("DECOMPILE FAILED {}".format(result.getErrorMessage()))
        continue
    print(result.getDecompiledFunction().getC())
    print("END {}".format(raw))
