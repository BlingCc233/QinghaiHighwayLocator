#@category OMAP

targets = ["141fb1c57", "141fad4a9", "141fc5899", "1421d1319", "1421d133b"]
for raw in targets:
    address = toAddr(long(raw, 16))
    function = getFunctionContaining(address)
    print("FUNCTION {} {}".format(raw, function))
    if function:
        print("ENTRY {} BODY {}".format(function.getEntryPoint(), function.getBody()))
    listing = currentProgram.getListing()
    instruction = listing.getInstructionAt(address)
    print("INSTRUCTION {}".format(instruction))
    if instruction:
        listing = currentProgram.getListing()
        scan = instruction.getAddress()
        for index in range(200):
            previous = listing.getInstructionBefore(scan)
            if previous is None:
                break
            scan = previous.getAddress()
            if previous.getMnemonicString() == "PUSH" and "RBP" in str(previous):
                scan = previous.getAddress().add(previous.getLength())
                break
        print("SCANSTART {}".format(scan))
        disassemble(scan)
        created = createFunction(scan, "writer_probe_" + raw)
        print("CREATED {}".format(created))
