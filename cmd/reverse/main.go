// Command reverse creates a disposable Ghidra project for the extracted loader.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// The Java source is generated only as a bridge to Ghidra's scripting API.
// It is a development artifact, never application code or a runtime dependency.
const bridge = `import ghidra.app.script.GhidraScript;
import ghidra.app.decompiler.*;
import ghidra.program.model.address.Address;
import ghidra.program.model.listing.*;
import java.io.*;
public class InspectBattleSquadron extends GhidraScript {
 public void run() throws Exception {
  String[] args = getScriptArgs();
  DecompInterface decompiler = new DecompInterface();
  decompiler.openProgram(currentProgram);
  try (PrintWriter out = new PrintWriter(args[0])) {
   for (int i = 1; i < args.length; i++) {
    Address entry = toAddr(Long.decode(args[i]));
    disassemble(entry);
    Function function = getFunctionAt(entry);
    if (function == null) function = createFunction(entry, "Original_" + args[i].replace("0x", ""));
    out.println("Original loader entry " + entry);
    Instruction instruction = getInstructionAt(entry);
    for (int j = 0; instruction != null && j < 160; j++) {
     out.println(instruction.getAddress() + " " + instruction);
     instruction = instruction.getNext();
    }
    if (function != null) {
     DecompileResults result = decompiler.decompileFunction(function, 30, monitor);
     if (result.decompileCompleted()) out.println(result.getDecompiledFunction().getC());
     else out.println(result.getErrorMessage());
    }
   }
  } finally { decompiler.dispose(); }
 }
}
`

func main() {
	loader := flag.String("loader", "assets/unpacked/loader.bin", "extracted original loader")
	addresses := flag.String("addresses", "0x100", "comma-separated original 68000 entry addresses")
	ghidra := flag.String("ghidra", "/opt/homebrew/opt/ghidra/libexec/support/analyzeHeadless", "Ghidra headless executable")
	output := flag.String("out", ".cache/reverse/loader.txt", "disassembly and decompiler output")
	flag.Parse()
	if err := run(*loader, *addresses, *ghidra, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(loader, addresses, ghidra, output string) error {
	loader, err := filepath.Abs(loader)
	if err != nil {
		return err
	}
	if _, err := os.Stat(loader); err != nil {
		return fmt.Errorf("extract the ADF first: %w", err)
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return err
	}
	work := filepath.Dir(output)
	if err := os.MkdirAll(work, 0755); err != nil {
		return err
	}
	script := filepath.Join(work, "InspectBattleSquadron.java")
	if err := os.WriteFile(script, []byte(bridge), 0644); err != nil {
		return err
	}
	// Ghidra rejects project paths containing a hidden directory component.
	project, err := os.MkdirTemp("", "battlesquadron-ghidra-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(project)
	args := []string{project, "BattleSquadron", "-deleteProject", "-import", loader, "-overwrite", "-noanalysis", "-loader", "BinaryLoader", "-loader-baseAddr", "0x100", "-processor", "68000:BE:32:default", "-scriptPath", work, "-postScript", "InspectBattleSquadron.java", output}
	for _, address := range strings.Split(addresses, ",") {
		args = append(args, strings.TrimSpace(address))
	}
	command := exec.Command(ghidra, args...)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	command.Env = os.Environ()
	// Keep Ghidra settings in the checkout along with its generated project.
	command.Env = append(command.Env, "JAVA_TOOL_OPTIONS=-Duser.home="+work)
	if os.Getenv("JAVA_HOME") == "" {
		for _, directory := range []string{"/opt/homebrew/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home", "/opt/homebrew/opt/openjdk/libexec/openjdk.jdk/Contents/Home"} {
			if _, err := os.Stat(directory); err == nil {
				command.Env = append(command.Env, "JAVA_HOME="+directory)
				break
			}
		}
	}
	return command.Run()
}
