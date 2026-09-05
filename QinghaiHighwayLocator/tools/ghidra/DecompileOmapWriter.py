#@category OMAP

from ghidra.app.decompiler import DecompInterface

decompiler = DecompInterface()
decompiler.openProgram(currentProgram)
function = getFunctionAt(toAddr(long("141fb1800", 16)))
print("BEGIN {}".format(function))
result = decompiler.decompileFunction(function, 180, monitor)
if not result.decompileCompleted():
    print("DECOMPILE FAILED {}".format(result.getErrorMessage()))
else:
    print(result.getDecompiledFunction().getC())
print("END")
