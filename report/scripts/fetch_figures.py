from pypdf import PdfReader, PdfWriter
import urllib.request
import os
import pdf2image
import fitz  # PyMuPDF


permalink = "https://docs.google.com/presentation/d/1RgcRZENtoHmxv5kiTlNwwcNEDgpMYzRaJOtbawGU5Vs/export?format=pdf"
first_slide = 2
last_slide = -1
pdf_local_name = "figures.pdf"
out_dir = "../img"



def fetch_pdf():
    urllib.request.urlretrieve(permalink, os.path.join("out" ,pdf_local_name))


def auto_crop_pdf(input_pdf, output_pdf):
    doc = fitz.open(input_pdf)
    for page in doc:
        # Get the union of all text blocks and drawing rectangles to find content bounds
        text_rects = [rect for block in page.get_text("dict")["blocks"] for rect in [block["bbox"]]]
        drawing_rects = [shape["rect"] for shape in page.get_drawings()]
        all_rects = text_rects + drawing_rects
        if all_rects:  # Check if there are any rects found
            # Calculate the bounding box around all found rectangles
            content_rect = fitz.Rect(all_rects[0])
            for rect in all_rects[1:]:
                # Ignore rect from 0.0, 0.0
                if rect[0] == 0.0 and rect[1] == 0.0:
                    continue

                content_rect.include_rect(fitz.Rect(rect))
            # Update the page's crop box to the content_rect
            page.set_cropbox(content_rect)

    doc.save(output_pdf)


def main():
    # Create dir out if not exists
    if not os.path.exists("out"):
        os.makedirs("out")

    print("Fetching figures...")
    fetch_pdf()

    print("Extracting figures...")
    reader = PdfReader(os.path.join("out" ,pdf_local_name))

    # The pdf should be structured as text>figure>text>figure>text>figure

    global last_slide
    if last_slide == -1:
        last_slide = len(reader.pages)
    
    for i in range(last_slide - first_slide):
        if i % 2 == 0:
            # Text, extract text
            filename = reader.pages[first_slide + i].extract_text()
            # Trim spaces around filename
            filename = filename.strip()
            print(filename)
        else:
            # Figure, split pdf and write to file
            # Then use pdf2image to convert to image
            # Then use imagemagick to convert to png
            writer = PdfWriter()
            writer.add_page(reader.pages[first_slide + i])
            
            writer.write("out/figure" + str(i) + ".pdf")

            auto_crop_pdf("out/figure" + str(i) + ".pdf", os.path.join(out_dir, filename + ".pdf"))

            # images = pdf2image.convert_from_path("out/figure" + str(i) + ".pdf")
            # images[0].save("out/figure" + str(i) + ".png", "PNG")

            # # Use imagemagick to crop unnecessary content around the image
            # output_file = os.path.join(out_dir, filename + ".png")
            # os.system("convert out/figure" + str(i) + ".png -trim " + output_file)

    # Remove out folder
    os.system("rm -rf out")

    print("Done")



if __name__ == "__main__":
    main()