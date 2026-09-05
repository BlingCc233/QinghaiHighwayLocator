#@category OMAP

from ghidra.program.model.address import Address
import jarray

targets = [
    "145b0bf30", # insert into object, main database writer
    "145b0ee00", # select object, transfer path
    "145b0ef20", # insert into object, transfer path
    "145b0f422", # PreLoadObjDbData OpenDB Cfgdb
    "145b0f4bb", # PreLoadObjDbData OpenDBToMemory
    "145b0f5c0", # select object in preload path
    "14781fd58", # pointer passed as the main object database key
    "146827828", # initial value of the main object database key pointer
]

for raw in targets:
    address = toAddr(long(raw, 16))
    memory = currentProgram.getMemory()
    rawBytes = jarray.zeros(24, 'b')
    memory.getBytes(address, rawBytes)
    print("TARGET {} {} {}".format(raw, getDataAt(address), ''.join('{:02x}'.format((value + 256) % 256) for value in rawBytes)))
    refs = getReferencesTo(address)
    print("REFS {}".format(len(refs)))
    for ref in refs:
        function = getFunctionContaining(ref.getFromAddress())
        print("  {} {} {}".format(ref.getFromAddress(), ref.getReferenceType(), function))
