using System.IO.Compression;
using System.Text;
using System.Xml.Linq;

if (args.Length != 2) throw new ArgumentException("Usage: NormalizePackage INPUT OUTPUT");
var inputPath = Path.GetFullPath(args[0]);
var outputPath = Path.GetFullPath(args[1]);
if (inputPath == outputPath) throw new ArgumentException("Input and output paths must differ.");

using var input = ZipFile.OpenRead(inputPath);
using var output = ZipFile.Open(outputPath, ZipArchiveMode.Create);
foreach (var source in input.Entries.OrderBy(entry => NormalizedName(entry.FullName), StringComparer.Ordinal))
{
    var name = NormalizedName(source.FullName);
    var target = output.CreateEntry(name, CompressionLevel.Optimal);
    target.LastWriteTime = new DateTimeOffset(1980, 1, 1, 0, 0, 0, TimeSpan.Zero);
    target.ExternalAttributes = unchecked((int)0x81A40000); // Unix regular file, mode 0644.
    await using var destination = target.Open();
    if (source.FullName == "_rels/.rels")
    {
        using var reader = new StreamReader(source.Open(), Encoding.UTF8);
        var document = XDocument.Parse(await reader.ReadToEndAsync());
        XNamespace relationships = "http://schemas.openxmlformats.org/package/2006/relationships";
        var core = document.Root?.Elements(relationships + "Relationship").Single(element =>
            element.Attribute("Type")?.Value.EndsWith("/metadata/core-properties", StringComparison.Ordinal) == true)
            ?? throw new InvalidDataException("Core-properties relationship is missing.");
        core.SetAttributeValue("Target", "/package/services/metadata/core-properties/ghanageo.psmdcp");
        core.SetAttributeValue("Id", "R_GHANAGEO_CORE_PROPERTIES");
        await using var writer = new StreamWriter(destination, new UTF8Encoding(false), leaveOpen: true);
        document.Save(writer, SaveOptions.DisableFormatting);
        await writer.FlushAsync();
    }
    else
    {
        await using var content = source.Open();
        await content.CopyToAsync(destination);
    }
}

static string NormalizedName(string name) =>
    name.StartsWith("package/services/metadata/core-properties/", StringComparison.Ordinal)
        ? "package/services/metadata/core-properties/ghanageo.psmdcp"
        : name;
